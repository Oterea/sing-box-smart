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
```

`app` 通过单写入事件循环编排探测、控制、快照、连接替换、选择同步和切换结果。基础模块并非完全互相独立：`state` 使用 `score`、`tiering`、`schedule`；`probe` 和 `switching` 使用 `gateway`。真实 API 实现在 `internal/real`，不是 `gateway` 包。

## app 文件边界

- `app.go`：类型、初始化、重置、请求/快照、公共 API 配置入口及恢复提示文字。
- `runtime.go`：事件循环、25ms 调度扫描、启动阶段、异步探测、结果处理。
- `control.go`：配置应用、手动节点/策略组选择、重新检查、暂停/继续入口。
- `internal/observewindow`：维护启动或恢复后的连续观察窗口，记录窗口时长和本轮尚未完成首次检查的节点。
- `detection.go`：取消探测及恢复检测计划。
- `switching.go`：选择比较、候选确认安排、写入前复核、异步写入和回读结果。
- `selection_mode.go`：自动/手动模式控制；模式只影响切换，不影响探测和计分。
- `selection_sync.go`：单实例读取根策略组和当前策略组的 `now`，处理外部 Dashboard 切换和运行状态重建。

这是同一个 Go package 内的函数拆分，不是多个独立服务。公共配置入口的网络发现发生在事件循环之外；运行状态通过快照读取，准备好的客户端通过请求安装。设置文件保存目前发生在事件循环内，磁盘等待仍可能阻塞该循环。

## 小模块边界

`score` 只计分；`tiering` 只划分候选；`recovery` 检测两类改善，接近候选边界由 `state` 判断；`schedule` 管到期时间；`decision` 返回切换目标；`switching` 执行并回读。候选确认结果先经过正常计分，再由 `app` 比较。

各模块实际阈值、调度行为和限制详见 [设计说明](design.md)。不将目标行为或单元测试结论当作已验证的运行保证。

## 并发和推送

每个节点最多一个在途探测，不同节点可并发。选择同步每 2 秒最多发起一组根策略组/当前策略组读取，由后端单实例完成，浏览器数量不会放大 sing-box API 请求。探测捕获 runner、上下文和 generation；旧机场或暂停前的结果会丢弃。切换请求使用根上下文，暂停不会撤销已发出的写入。

HTTP 快照通过请求通道从事件循环取得。SSE 连接共享一个快照采集器，每 250ms 最多向事件循环读取一次；各连接根据自己最后收到的快照计算节点差量，慢连接只保留最新版本。没有持久重放队列；浏览器合并 ID，历史通过前端队列播放。当前全组重置、旧节点移除的协议限制见 [接口文档](api.md)。

## OpenWrt 服务和连接设置

`internal/settings` 验证地址/正则并通过临时文件、Sync、rename 保存连接 JSON。完整连接设置通过一次 discovery 成功后再安装客户端和保存；`-settings` 内容优先覆盖 API/root/pattern；OpenWrt 使用 `/etc/sing-box-smart/connection.json`。服务启动配置是 `/etc/sing-box-smart.conf`。

LuCI JavaScript 通过 rpcd `file.exec` 调固定 init 命令；`service.list` 每 3 秒读取服务状态。开机自动启动状态在页面加载时读取，页面停止服务后仍可用。监控面板只支持修改连接和策略组过滤，不支持编辑探测间隔等全部检测参数。

`python3 scripts/build-ipk.py` 生成 ARM64 `aarch64_generic` 后端包和 `all` LuCI 包到 `dist/`，版本为脚本内的 0.2.0。这是构建步骤，修改源码或直接替换路由器二进制不会更新已有 ipk；使用前应重新构建。脚本和 SDK Makefile 都保留 `/etc/sing-box-smart/` 与 `/etc/sing-box-smart.conf`，升级时保留运行配置。请核对目标设备架构与 opkg 支持，不将 ipk 当 APK 使用。
