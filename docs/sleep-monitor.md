# gRPC 睡眠监控

睡眠监控只暂停 smart 的节点探测和自动切换，不停止 sing-box，也不会关闭正在使用的代理连接。

## 工作流程

```text
sing-box gRPC SubscribeConnections
        ↓
过滤 smart 自己的探测连接
        ↓
统计最近窗口内的真实流量
        ↓
最近 10 分钟低于 64 KiB
        ↓
暂停 smart 探测
        ↓ 用户重新产生连接
单个连接在 5 秒内累计至少 1 MiB 流量
        ↓
恢复当前节点检查和 5 秒快速观察
```

唤醒流量按连接 ID 分开累计。多个后台连接各自产生少量流量时不会合并触发唤醒；同一个连接持续传输达到阈值才会唤醒检测。没有连接 ID 的流量归入同一个未知连接。

gRPC 断开时活动源会自动重连，不主动发送睡眠指令。空闲判定仍基于已收到的流量；断线期间无法完整统计用户活动，不能将缺失的统计当作已验证的无人使用。

## 配置

推荐在 LuCI 服务设置中填写 gRPC 地址与密钥，在 9797 运行设置中启用并调整睡眠参数。下面是兼容的命令行初始值，面板保存值优先；详见 [面板设置](settings.md)。

```sh
-grpc 127.0.0.1:9090 \
-grpc-secret '你的 secret' \
-sleep-monitor=true \
-sleep-idle=10m \
-sleep-quiet-bytes=65536 \
-sleep-wake-window=5s \
-sleep-wake-bytes=1048576
```

OpenWrt 的配置文件对应 `GRPC_ADDR`、`GRPC_SECRET`、`SLEEP_MONITOR`、`SLEEP_IDLE`、`SLEEP_QUIET_BYTES`、`SLEEP_WAKE_WINDOW` 和 `SLEEP_WAKE_BYTES`。

必须先在 sing-box 中启用原生 `api` service。gRPC 地址建议只监听软路由本机地址，并配置 secret。

## 与手动暂停的关系

- 手动暂停优先级更高，不会被用户活动自动恢复。
- 手动继续检测会沿用原有的 5 秒快速观察流程。
- 自动睡眠恢复后也复用同一套恢复观察、计分和分层逻辑。
- 手动选择模式下可以继续探测，但恢复后不会自动切换节点。

## 模块边界

- `internal/activity`：定义与传输方式无关的活动事件。
- `internal/grpcactivity`：连接 sing-box gRPC，断线自动重连。
- `internal/sleep`：只负责流量窗口、阈值和 Active/Sleeping 状态转换。
- `internal/app/sleep.go`：把状态转换接入现有探测事件循环。
- `internal/singboxapi/daemon`：固定 sing-box 1.14 的 gRPC protobuf 定义。

当前 Clash API 仍负责策略组发现、延迟探测和节点切换。后续迁移这些功能时，只需增加 gRPC gateway 实现，不需要改睡眠状态机。
