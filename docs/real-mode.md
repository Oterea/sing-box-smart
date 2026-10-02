# 真实 sing-box 模式

程序调用 sing-box 的 Clash API；默认测试 URL 是 `https://www.gstatic.com/generate_204`。探测使用 API delay 接口，不是 ICMP ping。程序没有 Clash API secret 认证参数。

```sh
go run ./cmd/sing-box-smart \
  -mode real \
  -api http://127.0.0.1:9695 \
  -root proxy \
  -pins 'ToLink=tolink PIN,Pei=pei PIN' \
  -test-url https://www.gstatic.com/generate_204 \
  -listen 127.0.0.1:8787 \
  -settings connection.json
```

## 策略组发现

`-pins` 是可选兼容参数，格式为“显示名称=selector”。目录接口失败或动态目录没有匹配策略组时，它提供回退列表；动态目录成功找到匹配组时优先使用实际策略组。

目录读取成功时忽略 pins，读取 root 的直接成员，选出类型为 Selector 且名字符合正则的组；默认正则 `PIN$`，不要求 PIN 前有空格。此时显示名和 ID 都使用实际组名。root 不存在或没有成员时，代码会扫描整个目录匹配 Selector，因此当前并非始终只搜索 root 内部。

组成员应是待测节点；发现过程检查非空和重复，但不进一步保证每个成员都是单节点出站。只在启动、重新应用配置时发现，不会自动追踪后续订阅变化。

## 启动和切换

启动后选发现列表第一组，在该组内约 10 秒连续检查；所有节点至少完成一次后，提出最低分可用目标，确认探测并再次比较，再写入组 selector。启动不会自动把根组切到这个 PIN。

手动切换策略组时写入当前配置的根策略组；确认后重新执行启动观察。正常间隔、恢复及失败处理见 [设计说明](design.md)。

HTTP 节点失败响应和外层超时计为节点失败；其他连接错误计为 API 异常。不能保证所有错误 HTTP 响应都被正确识别为管理接口故障。

## OpenWrt

示例服务配置 `deploy/sing-box-smart.conf` 监听 `0.0.0.0:9797`，调用本机 API `127.0.0.1:9695`。LuCI 管理 init 服务启动、停止、重启、自启动；监控页面暂停/继续检测不等于停止进程。

已有 `/etc/sing-box-smart/connection.json` 会优先覆盖启动配置中的 API、root、pattern。重启会清空评分和历史，保留该连接文件。参数未提供定时间隔的 CLI 开关，当前间隔在 `internal/config.Default` 中配置。

`deploy/install-router.sh` 是早期 sbwatch 迁移脚本，会备份、停止并禁用 sbwatch，再安装指定 /tmp 文件；不是通用 ipk 安装步骤。
