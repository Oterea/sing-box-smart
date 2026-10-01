# HTTP API

## 状态与推送

- `GET /api/state`：完整快照，包括 `revision`、策略组、节点、阶段、连接设置及最近事件。响应包含 ETag；匹配 `If-None-Match` 时返回 304。
- `GET /api/events`：`text/event-stream`，事件名为 `state`，`data` 为 JSON。每条连接每 250ms 获取快照，仅在版本变化时发送。

SSE 首条消息包含 `full:true` 和全部节点，后续 `full:false` 的 `nodes` 只含与该连接上次快照相比发生变化的节点，`removed` 列出消失的节点 ID。策略组变化会重新发送完整状态。其他全局字段仍完整发送，没有心跳、持久事件 ID 或断点重放。重新连接会重新发送完整状态。

ETag 只有计数，未带进程身份；重启后计数可能重复，但 SSE 重连或策略组变化会发送完整状态。

## 控制

以下接口返回 202 表示请求已受理，不代表异步切换已完成；失败通常返回 409，错误信息在 `error` 字段中。

| 接口 | JSON 请求体 | 行为 |
| --- | --- | --- |
| `POST /api/control/airport` | `{ "id": "tolink PIN" }` | 选择策略组，确认后重新启动观察；当前写入目标根组固定为 `proxy` |
| `POST /api/control/node` | `{ "id": "实际节点名称" }` | 探测目标，成功后手动选择，不受自动 1.4 倍门槛约束 |
| `POST /api/control/recheck` | `{}` | 将当前组所有节点安排为到期；已有在途探测不重复启动 |
| `POST /api/control/pause` | `{}` | 暂停新探测、丢弃旧结果、不再发起自动切换 |
| `POST /api/control/resume` | `{}` | 继续检测并安排全组重新检查 |

暂停不会撤销已经发出的 selector 写入；该写入仍可能完成并回读。启动阶段拒绝手动节点选择。

## 连接设置

| 接口 | JSON 请求体 | 行为 |
| --- | --- | --- |
| `POST /api/config/api/test` | `{ "api": "http://127.0.0.1:9695" }` | 用独立客户端读取策略组，成功返回 200，不替换活动连接 |
| `POST /api/config/api` | `{ "api": "http://127.0.0.1:9695" }` | 验证可读取、保存连接、替换客户端、重置启动观察 |
| `POST /api/config/filters` | `{ "root": "proxy", "pattern": "PIN$" }` | 验证正则、读取匹配策略组、保存并重置启动观察 |

面板保存动作目前依次调用 filters 和 api 两个接口，不是原子事务；只要两次都成功，就会重置两次。若同时更换地址及正则，filters 先在旧地址上验证。

## 访问范围

本地默认监听回环 `127.0.0.1:8787`；OpenWrt 示例监听 `0.0.0.0:9797`。POST 对携带的 Origin 做同源校验；控制接口还要求该 Origin 使用 HTTP。不存在内置登录或 API secret 支持，不能把同源检查描述为认证。
