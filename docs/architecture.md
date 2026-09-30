# 架构说明

## 数据流

```text
gateway -> discovery -> app 调度 -> probe -> state -> score
                                      -> decision -> switching -> gateway
                                                        -> snapshot -> httpapi -> web
```

`app` 是唯一能同时看见调度、状态和切换状态的编排层。各基础模块不互相调用，便于单独测试和替换。

## 小模块的边界

每个模块只有一个原因需要修改：探测改动在 `probe`，公式改动在 `score`，分层改动在 `tiering`，恢复迹象判断在 `recovery`，检查时间改动在 `schedule`，sing-box API 改动在 `gateway`。模块小，但不为了“一个函数一个包”而过度拆分。

`decision` 只返回“建议切换到谁”；`switching` 只执行并确认接口动作。候选确认由 `app` 调用一次 `probe` 完成，确认结果先进入同一个 `state`，随后再次比较。

`tiering` 每次节点结果完成后按当前可用节点的分数分布寻找明显断层，动态得到优秀候选和普通节点；它不使用固定前 N。没有可信断层时保留相近的可用节点为候选。`state` 将当前节点强制标记为当前层，但当前节点的 1.4 倍切换判断仍由 `decision` 独立完成。

`recovery` 只为低频普通节点寻找两类恢复迹象：失败后的成功、相对近期成功延迟基线明显下降。它不改分数，只让该节点获得 3、6、12、20 秒的额外观测机会；每次观测仍经过 `state` 和 `score` 的正常路径。

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

## OpenWrt service management and connection settings

`internal/settings` validates URLs and writes the connection file by atomic rename.
The `-settings` file overrides the API startup flag when present. OpenWrt uses
`/etc/sing-box-smart/connection.json`. The existing service configuration remains
`/etc/sing-box-smart.conf`; both paths are preserved by the upgrade keep list.

Connection testing creates a separate client. Network discovery runs outside the
app event loop. Applying a connection saves it first, then installs the prepared
client through the event loop, cancels old probes and resets startup observation.
An invalid or unreachable endpoint never replaces the active client.

The LuCI JavaScript view calls the existing init script through rpcd file.exec,
with ACL entries restricted to fixed service commands. procd service.list supplies
status every three seconds. LuCI is available when the smart process is stopped.
No custom rpcd daemon is needed for these operations.

`python3 scripts/build-ipk.py` builds ARM64 opkg packages into `dist/` for this
router. These artifacts are for OpenWrt/ImmortalWrt 24.10 aarch64_generic; they
must not be used as APK packages or on another CPU architecture. The LuCI source
is separately packaged as `luci-app-sing-box-smart` (architecture `all`).
