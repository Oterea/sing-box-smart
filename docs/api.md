# HTTP API（原型）

- `GET /api/state`：返回当前机场、节点视图、启动阶段和最近事件。
- `POST /api/control/airport`，body `{ "id": "airport-1" }`：切换机场 PIN；后端重新执行启动观察。
- `POST /api/control/node`，body `{ "id": "airport-1-node-02" }`：先确认目标，再手动切换。
- `POST /api/control/recheck`，body `{}`：安排当前机场全部节点立即检查。

原型只监听回环地址，并拒绝带有不匹配 Origin 的控制请求。真实部署时还需要接入认证与 CSRF 策略。
