# PushBot Go 版

本仓库已去掉 shell 脚本，**仅使用 Go 实现**。与原有配置兼容：同一套 UCI 与 LuCI 界面，OUI 数据位于 `/usr/share/pushbot/oui.txt`。

## 功能对应

| 功能           | 说明 |
|----------------|------|
| 设备上下线     | 基于 ARP + ping/arping 检测，上线/下线推送 |
| 公网 IP 变化   | 支持接口获取与 URL 获取 IPv4/IPv6 |
| 推送           | 使用现有 api/*.json 模板（钉钉、PushPlus、Bark 等） |
| 定时推送       | regular_time / interval_time |
| 免打扰         | starttime/endtime 内不推送 |
| 无人值守       | 关注列表均离线时可执行重启/重新拨号 |
| CPU 负载       | 读取 /proc/loadavg 超阈值推送 |
| 在线设备列表   | 推送内容中附带当前在线设备；`pushbot -client` 输出 HTML |

## 依赖

- **运行**：仅需 UCI（OpenWrt 自带）。推送需 curl 或本程序内置 HTTP 即可（已用标准库）。
- **设备检测**：建议安装 `iputils-arping`，否则仅用 ping。
- **配置**：与 LuCI 一致，需先安装 `luci-app-pushbot` 并配置好 UCI 与 api/*.json。

## 编译

```bash
# 本机
go build -o pushbot .

# OpenWrt 常见架构（在 PC 上交叉编译）
make arm    # 例如 ARM 路由
make mips   # 例如 MIPS 路由
```

编译完成后将对应二进制放到路由器：

```bash
# 替换原脚本（备份后）
cp pushbot /usr/bin/pushbot/pushbot
chmod +x /usr/bin/pushbot/pushbot
```

原 init 脚本启动的是 `/usr/bin/pushbot/pushbot`，若该路径是 Go 二进制，则会以 Go 版运行。

## 运行方式

- **后台服务**：由 `/etc/init.d/pushbot` 启动。
- **单次定时推送**：`/usr/bin/pushbot/pushbot -send` 或 `pushbot send`（LuCI/cron 兼容）
- **输出在线设备 HTML**：`pushbot -client` / `pushbot client`
- **测试推送**：`pushbot -test` / `pushbot test`
- **温度（LuCI 高级设置）**：`pushbot soc`，结果写入 `/tmp/pushbot/soc_tmp`

## 轮询与唤醒说明（已优化，减少空转）

- **主循环**：不再固定每 N 秒轮询，而是 **睡眠到「下次任务到期」或「有待发送通知」** 再执行。
  - **按需唤醒**：neightmon 产生上下线通知时会往 `wakeCh` 发信号，主循环立即醒来执行发送，不空转。
  - **到点执行**：公网 IP、CPU 负载、无人值守、定时推送 均设「下次执行时间」，只有当前时间 ≥ 下次时间才执行，执行后再推到 5 分钟（或下次整点）后。
- **间隔**：公网 IP / CPU 负载 / 无人值守 默认 **5 分钟** 执行一次；定时推送按配置的整点或间隔整点只在那刻检查。
- **设备上下线**：若已启用 Netlink 邻居表事件，由事件驱动；否则由主循环到点 + `sendLockPath` 触发 `pushbotFirst`。

## 与 Shell 版差异

- **登录提醒**（Web/SSH 登录/失败）：当前未实现，后续可通过 logread 或 syslog 接入。
- **温度**：`pushbot soc` 已实现，写温度到 /tmp/pushbot/soc_tmp（LuCI 高级设置「测试温度命令」）。
- **OUI 厂商名**：从 /usr/share/pushbot/oui.txt（或 oui_base.txt）及 /usr/bin/pushbot/ 下同名文件读取；不包含自动下载逻辑。
- **LuCI 客户端页**：`-client` 输出简化表格（客户端名/MAC/IP/在线时间），无流量列。

## 配置路径

- 配置：UCI `pushbot.@pushbot[0].*`
- 工作目录：`/tmp/pushbot/`（ipAddress、ip、pushbot.log、soc_tmp 等）
- 模板与列表：`/usr/bin/pushbot/api/*.json`、`ipv4.list`、`ipv6.list`
- OUI 数据：`/usr/share/pushbot/oui.txt`（包内安装），可选 /usr/bin/pushbot/oui_base.txt
