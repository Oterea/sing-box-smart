# 架构说明

## 数据流

```text
gateway -> discovery -> app 调度 -> probe -> state -> score
                                      -> decision -> switching -> gateway
                                                        -> snapshot -> httpapi -> web
```

`app` 是唯一能同时看见调度、状态和切换状态的编排层。各基础模块不互相调用，便于单独测试和替换。

## 小模块的边界

每个模块只有一个原因需要修改：探测改动在 `probe`，公式改动在 `score`，检查时间改动在 `schedule`，sing-box API 改动在 `gateway`。模块小，但不为了“一个函数一个包”而过度拆分。

`decision` 只返回“建议切换到谁”；`switching` 只执行并确认接口动作。候选确认由 `app` 调用一次 `probe` 完成，确认结果先进入同一个 `state`，随后再次比较。

## 并发

每个节点最多一个进行中的探测。探测可以并发；结果通过事件通道回到 `app` 的单写入循环。旧机场或旧一代探测返回后会被丢弃，避免切换机场后污染新状态。

## 真实接口接入

替换 `internal/demo`，实现 `gateway.Client`：

1. 从 sing-box 的 selector 读取 `proxy` 与各机场 PIN 的成员。
2. 对单个成员执行 delay 探测。
3. 选择前确认 selector 归属。
4. PUT 选择请求后 GET 读取实际选择。
5. 管理接口健康错误与节点探测失败必须分开返回；不能把 API 中断批量记成节点失败。

认证、地址和 selector 映射放进配置，不进入评分模块，也不写入日志。
