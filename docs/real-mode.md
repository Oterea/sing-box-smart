# 真实 sing-box 模式

程序使用 sing-box 的 Clash API。先确认 sing-box 的 external controller 只监听本机，再启动程序。默认不会监听外部网卡。

```sh
go run ./cmd/sing-box-smart \
  -mode real \
  -api http://127.0.0.1:9695 \
  -root proxy \
  -pins 'Pei=pei PIN,ToLink=tolink PIN,Cloud=cloud PIN' \
  -test-url https://www.gstatic.com/generate_204 \
  -listen 127.0.0.1:8787
```

`-pins` 的左边只是面板显示名，右边必须是 sing-box 中真实存在的 selector 名称。每个 PIN selector 的成员应是该机场的节点。

启动时程序会分别读取每个 PIN 的成员，约 10 秒内独立检查，随后从有成功延迟的节点中选最低分节点写入该 PIN。切换机场时，程序向根 selector（默认 `proxy`）选择对应的 PIN；选择机场后重新执行启动观察。

接口连接错误不会记为节点失败。delay 接口能正常返回 HTTP 响应但表示节点失败时，才把这次结果记为节点失败。

当前版本不保存评分状态跨重启。日志只写程序事件和探测摘要，不写 API 密钥；如果未来给 API 加认证，需要把认证配置放在本地权限受限的配置文件或系统密钥存储中。
