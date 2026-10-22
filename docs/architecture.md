# 架构说明

## 数据流与模块依赖

```text
real/demo（gateway.Client） → discovery → app
app → probe → gateway.Client → 探测结果通道 → app
app → state.Record → score
app → state.Reclassify → tiering
app → recovery / state.NearCandidate → schedule
app → decision → 确认探测 → switching → selector 写入与回读
app → snapshot → httpapi（HTTP / SSE） → web
sing-box gRPC → grpcactivity → activity → app → sleep controller → observation gate
```

`app` 通过单写入事件循环编排探测、控制、快照、连接替换、选择同步和切换结果。基础模块并非完全互相独立：`state` 使用 `score`、`tiering`、`schedule`；`probe` 和 `switching` 使用 `gateway`。真实 API 实现在 `internal/real`，不是 `gateway` 包。

长期稳定性由 `stability.LongTerm` 聚合，持久化通过 `LongTermPersistence` 接口隔离；当前 JSON 实现只保存最近 14 天实际有数据的 15 分钟时间桶。`app` 只依赖聚合接口，不直接依赖文件格式。

## 时间边界与日历模块

系统把时间分成两类处理：探测超时、调度间隔、睡眠窗口和切换确认使用 `time.Duration`；探测事件、日志和 SSE 使用带时区的 `time.Time` 瞬时值；只有“属于哪一天、当天几点、落在哪个 15 分钟桶”才进入 `internal/clock.Calendar`。这样服务器运行在哪个系统时区不会改变用户看到的日期。

`config.Config.Timezone` 默认是 `Asia/Shanghai`，可通过 `--timezone` 显式覆盖。`stability.LongTerm` 只依赖注入的 `clock.Calendar`，不读取 `time.Local`；它在后端计算 `date`、`minute_of_day` 和桶起点，HTTP API 同时返回 `timezone`。浏览器趋势图只使用 `minute_of_day`，不再用浏览器自己的时区重新解析 `at`。长期聚合持久化版本为 v2，旧版按服务器 `time.Local` 分桶，无法安全迁移，会自动重新积累。

这套边界是可拼接的：将来需要可测试的当前时间时，可在 `internal/clock` 增加注入式 `Clock`，而不改变 `Calendar`、长期聚合、API 或前端的职责；持续时间和瞬时事件也不需要被改写成日历时间。

睡眠监控通过 gRPC 的连接事件独立接入。`grpcactivity` 不参与评分和节点选择，只把用户连接活动转换成 `activity.Event`；`sleep` 只维护空闲状态；最终由 `app` 事件循环决定是否暂停或恢复探测。这样后续将策略组、切换和探测迁移到 gRPC 时，不需要复制睡眠逻辑。

## app 文件边界

- `app.go`：类型、初始化、重置、请求/快照、公共 API 配置入口及恢复提示文字。
- `runtime.go`：事件循环、25ms 调度扫描、启动阶段、异步探测、结果处理。
- `control.go`：配置应用、手动节点/策略组选择、重新检查、暂停/继续入口。
- `internal/observewindow`：维护启动或恢复后的连续观察窗口，记录窗口时长和本轮尚未完成首次检查的节点。
- `detection.go`：取消探测及恢复检测计划。
- `preferences.go`：在事件循环内校验、保存并应用运行设置。
- `activity_source.go`：启动、替换和取消 gRPC 活动源。
- `switching.go`：选择比较、候选确认安排、写入前复核、异步写入和回读结果。
- `internal/switching/confirmation.go`：维护最多 3 次、15 秒总上限的节点切换确认批次；不负责启动探测，只提供确认状态。
- `internal/stability`：按节点维护最近 24 小时的内存探测窗口，计算 P50、P90、成功率和失败连续性；普通状态只读取摘要，详情接口才复制原始样本。
- `selection_mode.go`：自动/手动模式控制；模式只影响切换，不影响探测和计分。
- `selection_sync.go`：单实例读取根策略组和当前策略组的 `now`，处理外部 Dashboard 切换和运行状态重建。

这是同一个 Go package 内的函数拆分，不是多个独立服务。公共配置入口的网络发现发生在事件循环之外；运行状态通过快照读取，准备好的客户端通过请求安装。设置文件保存目前发生在事件循环内，磁盘等待仍可能阻塞该循环。

## 小模块边界

`score` 只计分；`tiering` 只划分候选；`recovery` 检测两类改善，接近候选边界由 `state` 判断；`schedule` 管到期时间；`decision` 返回切换目标；`switching` 执行并回读。候选确认结果先经过正常计分，再由 `app` 比较。

浏览器资源按职责拆分：`node-history.js` 负责主面板历史图，`node-detail.js` 负责 24 小时实时详情和 14 天趋势，`stability-view.js` 只提供跨视图共享的稳定性结论规则；`app.js` 负责页面装配、主状态和事件协调。

长期比较由 `stability.LongTerm.Overview` 从已有直方图一次聚合 1、3、7、14 天窗口，`GET /api/nodes/stability/overview` 批量返回当前策略组数据；`longterm-overview.js` 只负责窗口选择、分组、组内多指标排序和排名渲染，并复用节点表格的滚动和响应式布局。这样详情图和主面板比较不会互相放大请求，也不会让浏览器重复计算长期统计。

各模块实际阈值、调度行为和限制详见 [设计说明](design.md)。不将目标行为或单元测试结论当作已验证的运行保证。

## 并发和推送

每个节点最多一个在途探测，不同节点可并发。选择同步每 2 秒最多发起一组根策略组/当前策略组读取，由后端单实例完成，浏览器数量不会放大 sing-box API 请求。探测捕获 runner、上下文和 generation；旧机场或暂停前的结果会丢弃。切换请求使用根上下文，暂停不会撤销已发出的写入。

HTTP 快照通过请求通道从事件循环取得。SSE 连接共享一个快照采集器，每 250ms 最多向事件循环读取一次；各连接根据自己最后收到的快照计算节点差量，慢连接只保留最新版本。没有持久重放队列；浏览器合并 ID，历史通过前端队列播放。当前全组重置、旧节点移除的协议限制见 [接口文档](api.md)。

## OpenWrt 服务和连接设置

`internal/settings` 验证地址/正则并通过临时文件、Sync、rename 保存连接 JSON。完整连接设置通过一次 discovery 成功后再安装客户端和保存；`-settings` 内容优先覆盖 API/root/pattern；OpenWrt 使用 `/etc/sing-box-smart/connection.json`。服务启动配置是 `/etc/sing-box-smart.conf`，面板保存的 `runtime.json` 和 `service.json` 分别覆盖对应运行参数与服务参数。详见 [面板设置](settings.md)。

LuCI JavaScript 通过 rpcd `file.exec` 调固定 init 命令；`service.list` 每 3 秒读取服务状态。开机自动启动状态在页面加载时读取，页面停止服务后仍可用。监控面板提供连接设置与运行设置；运行设置可热更新检测、睡眠、日志级别和延迟颜色。LuCI 通过固定命令校验并保存服务设置，重启 smart 后生效；设置文件不作为 shell 脚本执行。

`python3 scripts/build-ipk.py` 生成 ARM64 `aarch64_generic` 后端包和 `all` LuCI 包到 `dist/`，版本为脚本内的 0.2.0。这是构建步骤，修改源码或直接替换路由器二进制不会更新已有 ipk；使用前应重新构建。脚本和 SDK Makefile 都保留 `/etc/sing-box-smart/` 与 `/etc/sing-box-smart.conf`，升级时保留运行配置。请核对目标设备架构与 opkg 支持，不将 ipk 当 APK 使用。
