# gRPC 睡眠监控

睡眠监控只暂停 smart 的节点探测和自动切换，不停止 sing-box，也不会关闭正在使用的代理连接。

## 工作流程

```text
sing-box gRPC SubscribeConnections
        ↓
过滤 smart 自己的探测连接
        ↓
判断用户是否产生新连接或流量
        ↓
空闲达到 sleep-idle
        ↓
暂停 smart 探测
        ↓ 用户重新产生连接
恢复当前节点检查和 5 秒快速观察
```

gRPC 连接断开时不会被当作“没有用户活动”，smart 会保持现有探测策略并等待重连，避免 API 故障导致误睡眠。

## 配置

```sh
-grpc 127.0.0.1:9090 \
-grpc-secret '你的 secret' \
-sleep-monitor=true \
-sleep-idle=10m
```

OpenWrt 的配置文件对应 `GRPC_ADDR`、`GRPC_SECRET`、`SLEEP_MONITOR` 和 `SLEEP_IDLE`。

必须先在 sing-box 中启用原生 `api` service。gRPC 地址建议只监听软路由本机地址，并配置 secret。

## 与手动暂停的关系

- 手动暂停优先级更高，不会被用户活动自动恢复。
- 手动继续检测会沿用原有的 5 秒快速观察流程。
- 自动睡眠恢复后也复用同一套恢复观察、计分和分层逻辑。
- 手动选择模式下可以继续探测，但恢复后不会自动切换节点。

## 模块边界

- `internal/activity`：定义与传输方式无关的活动事件。
- `internal/grpcactivity`：连接 sing-box gRPC，断线自动重连。
- `internal/sleep`：只负责空闲计时和 Active/Sleeping 状态转换。
- `internal/app/sleep.go`：把状态转换接入现有探测事件循环。
- `internal/singboxapi/daemon`：固定 sing-box 1.14 的 gRPC protobuf 定义。

当前 Clash API 仍负责策略组发现、延迟探测和节点切换。后续迁移这些功能时，只需增加 gRPC gateway 实现，不需要改睡眠状态机。
