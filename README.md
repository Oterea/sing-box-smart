# sing-box-smart

sing-box 节点探测、评分、自动选择及 OpenWrt 服务管理程序。后端使用 Go，监控面板使用嵌入式 HTML/CSS/JavaScript，LuCI 页面负责服务启动、停止、重启及开机自启动。

## 运行与验证

```sh
make run                 # 模拟模式，http://127.0.0.1:8787
make test                # Go 测试及竞态检测
make check               # go vet
node --test tests/*.cjs   # 前端测试，需要可用的 Node.js
```

真实接入见 [真实模式](docs/real-mode.md)，接口见 [HTTP API](docs/api.md)，行为与已知限制见 [设计说明](docs/design.md)。测试通过表示现有测试覆盖的场景通过，不表示所有边界或浏览器视觉效果已经验证。

## 目录

- `cmd/sing-box-smart`：启动参数、组件组装、HTTP 服务生命周期。
- `internal/domain`：节点、探测结果、历史、面板快照等数据结构。
- `internal/gateway`：真实适配器和模拟适配器共同实现的接口。
- `internal/demo`：用于本地演示和测试的模拟网关。
- `internal/real`：sing-box Clash API 适配器。
- `internal/discovery`：策略组与节点身份、重复及归属校验。
- `internal/probe`：执行一次带超时的探测。
- `internal/score`：评分公式，失败不伪造延迟。
- `internal/state`：内存指标、有限历史、分层结果与调度更新。
- `internal/tiering`：候选层划分及最多 20 个的上限。
- `internal/recovery`：失败后低延迟成功、明显降延迟的判断。
- `internal/schedule`：探测到期时间、恢复复查步骤。
- `internal/decision`：比较可用节点，提出切换目标。
- `internal/switching`：写入 selector 并读取实际选择。
- `internal/events`：最近事件、异步滚动文本日志。
- `internal/settings`：验证并持久化 API 地址、根策略组、匹配正则。
- `internal/app`：通过单写入事件循环组合各模块。
- `internal/httpapi`：状态、控制、配置、SSE 和静态资源接口。
- `web`：监控面板；`package`：OpenWrt/LuCI 包源码；`deploy`：服务脚本和示例配置。

## 当前行为摘要

- 启动时只探测当前所选策略组的节点；每个节点独立连续探测。至少经过约 10 秒且所有节点都完成过一次探测后，开始选择并确认目标。
- 正常间隔：当前节点 3 秒、候选节点 30 秒、普通节点 5 分钟。间隔通常从上一次探测完成算起，不是固定开始时间周期。全组重排时会错峰。
- 分层寻找明显分数断层；无断层且可用节点超过 20 时，选前 `max(10, floor(可用数/4))`，最终最多 20 个。当前节点单独标记。
- 普通节点的恢复信号包括：失败后成功且延迟不超过 400ms；近期成功延迟中位基线至少下降 100ms、且降至基线的 75% 以下；计分后仍处普通层且分数不超过候选边界的 1.15 倍。
- 恢复计划配置为 3、6、12、20 秒；未到恢复时间时不会执行普通探测，每次结果照常计分。
- 分数越低越好：`L / (p³ × (1-f)³)`，`p=(S+1)/(N+2)`。失败不更新 `L`。
- 评分和历史不跨重启保存。后端每节点保留 24 条历史，面板显示最近 20 条。连接设置持久保存。
- 监控面板优先使用 SSE；所有连接共享每 250ms 一次的快照读取，版本不变不发送。首次发送完整状态，之后发送变化节点及全局字段；慢客户端只保留最新版本。页面按实例 ID 和 revision 丢弃旧响应，断线或超过 3 秒无有效状态事件时使用轮询备用。
- 日志路径默认 `logs/events.jsonl`，OpenWrt 示例为 `/var/log/sing-box-smart/events.jsonl`。文件内容是带时间戳的文本，虽然扩展名是 `.jsonl`，但并非 JSONL；约 5 MiB 滚动，保留一个 `.1` 备份。
