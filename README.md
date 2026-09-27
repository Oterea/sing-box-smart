# sing-box-smart

sing-box 节点检查与自动选择的模块化原型。

当前版本使用模拟网关运行完整流程，默认监听 `127.0.0.1:8787`。它用于验证启动、评分、复查、选择和面板交互；不会连接或修改真实 sing-box。

## 运行

```sh
make run
# 浏览器打开 http://127.0.0.1:8787
```

可选参数：

```sh
go run ./cmd/sing-box-smart -listen 127.0.0.1:8787 -log-dir logs
```

## 验证

```sh
make test
make check
```

## 目录

- `cmd/sing-box-smart`：唯一的启动入口，只负责配置、组装和生命周期。
- `internal/domain`：节点、机场、探测结果和面板快照等纯数据结构。
- `internal/gateway`：sing-box 的最小边界；真实 HTTP/API 适配器以后放在这里。
- `internal/demo`：模拟 API，和真实适配器使用同一个接口。
- `internal/discovery`：读取机场 PIN 与节点，校验身份和归属。
- `internal/probe`：执行一次有超时的探测，不保存历史、不决定切换。
- `internal/score`：只实现公式，失败不伪造延迟。
- `internal/state`：保存内存中的节点指标和有限历史；重启清空。
- `internal/schedule`：决定什么时候检查，不负责评分和切换。
- `internal/decision`：纯比较函数，按 1.4 倍规则提出候选。
- `internal/switching`：执行选择并读取实际选择结果，不修改评分。
- `internal/events`：界面最近事件和 JSONL 运行日志。
- `internal/app`：把各小模块编排成一个单写入循环。
- `internal/httpapi`：只提供状态读取和控制入口。
- `web`：嵌入式浅色 HTML/CSS/JS 面板。
- `docs`：架构、接口和设计约定。

## 当前实现的行为

- 启动时所有节点各自立即连续检查，约 10 秒内不选择；所有节点至少完成一次检查后选择可用节点中分数最低者。
- 正常频率：当前节点 2 秒、其他节点 30 秒；上一次失败后恢复的候选临时以 3 秒复查，最多 20 次。
- 公式：`L / (p³ × (1-f)³)`，其中 `p=(S+1)/(N+2)`，失败不写入 `L`。
- 分数只在本次运行保存；日志写入 `logs/events.jsonl`，文件约 5 MiB 后滚动一个备份。
- 面板显示当前机场 PIN 的节点、分数历史、延迟历史、最近事件和手动切换入口。
