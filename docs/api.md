# HTTP API

## 状态与推送

- `GET /api/state`：完整快照，包括 `instance_id`、`revision`、策略组、节点、阶段、连接设置及最近事件。响应包含带实例 ID 的弱 ETag；匹配 `If-None-Match` 时返回 304。
- `GET /api/events`：`text/event-stream`，事件名为 `state`，`data` 为 JSON。服务端在所有 SSE 连接之间共享一次每 250ms 的快照采集；版本没有变化就不发送状态事件。

SSE 首条消息包含 `full:true` 和全部节点，后续 `full:false` 的 `nodes` 只含与该连接上次收到的快照相比发生变化的节点，`removed` 列出消失的节点 ID。慢客户端只保留最新快照，可能跳过中间版本，但不会丢失最终状态。策略组变化会重新发送完整状态。状态事件带 `id`，值为 `revision`；另有 15 秒注释心跳用于保持连接，没有断点重放，重新连接会重新发送完整状态。服务端发送 `retry: 3000` 的重连建议由浏览器 EventSource 使用。

`instance_id` 在 HTTP 服务实例启动时生成，`revision` 只在该实例内递增；两者一起构成 ETag 和 SSE 状态身份。重启后会生成新实例，SSE 重连会发送完整状态。

## 控制

以下接口返回 202 表示请求已受理，不代表异步切换已完成；失败通常返回 409，错误信息在 `error` 字段中。

| 接口 | JSON 请求体 | 行为 |
| --- | --- | --- |
| `POST /api/control/airport` | `{ "id": "tolink PIN" }` | 选择策略组，确认后重新启动观察；写入当前配置的根策略组 |
| `POST /api/control/node` | `{ "id": "实际节点名称" }` | 探测目标，成功后手动选择，不受自动 1.4 倍门槛约束；检测继续运行，但不会再自动切换 |
| `POST /api/control/auto` | `{}` | 开启 smart 自动选择；保持当前节点，后续按正常评分和切换条件决定是否切换 |
| `POST /api/control/manual` | `{}` | 开启手动选择；保持 sing-box 当前节点，继续检测和更新数据 |
| `POST /api/control/recheck` | `{}` | 将当前组所有节点安排为到期；已有在途探测不重复启动 |
| `POST /api/control/pause` | `{}` | 暂停新探测、丢弃旧结果、不再发起自动切换 |
| `POST /api/control/resume` | `{}` | 继续检测；先并发检查当前组每个节点一次，全部完成后恢复分层调度并进行一次自动判断 |

暂停不会撤销已经发出的 selector 写入；该写入仍可能完成并回读。启动阶段拒绝手动节点选择。

## 选择状态同步

后端运行循环每 2 秒读取一次根策略组和当前机场策略组的 `now` 值：

- smart 写入 selector 后，仍通过 Clash API 回读确认，面板和 sing-box Dashboard 显示同一节点；
- 用户在 sing-box Dashboard 切换机场或节点后，smart 会更新当前机场、当前节点、分层和检查计划；
- 检测到外部切换时自动进入手动模式，避免 smart 把用户刚选的节点立即改回去；
- 同步由单个 app 事件循环负责，SSE 客户端不会直接访问 sing-box，也不会因打开多个面板而增加 API 轮询次数；
- 状态读取失败不会伪造节点失败，下一轮继续同步；节点探测 API 的健康状态仍由原有健康检查负责。

## 连接设置

| 接口 | JSON 请求体 | 行为 |
| --- | --- | --- |
| `POST /api/config` | `{ "api": "http://127.0.0.1:9695", "root": "proxy", "pattern": "PIN$" }` | 一次验证、发现并应用完整连接设置；推荐面板使用此接口 |
| `POST /api/config/api/test` | `{ "api": "http://127.0.0.1:9695" }` | 用独立客户端读取策略组，成功返回 200，不替换活动连接 |
| `POST /api/config/api` | `{ "api": "http://127.0.0.1:9695" }` | 验证可读取、保存连接、替换客户端、重置启动观察 |
| `POST /api/config/filters` | `{ "root": "proxy", "pattern": "PIN$" }` | 验证正则、读取匹配策略组、保存并重置启动观察 |

`/api/config/api` 和 `/api/config/filters` 保留用于兼容分步调用；面板使用 `/api/config`，避免地址和策略组设置分两次生效。完整接口只有在新客户端发现成功后才替换当前连接。

## 访问范围

本地默认监听回环 `127.0.0.1:8787`；OpenWrt 示例监听 `0.0.0.0:9797`。POST 对携带的 Origin 做同源校验，并接受 HTTP 或 HTTPS 同源页面。不存在内置登录或 API secret 支持，不能把同源检查描述为认证。
