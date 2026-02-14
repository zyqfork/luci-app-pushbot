# 编译说明（C 版）
- 本仓库已**去掉 shell 脚本**，主程序为 **pushbot-c** 编译的 C 二进制，无需 Go 环境。
- OUI 数据位于 **`/usr/share/pushbot/oui.txt`**，由包内安装。

## 重要：为何 `make package/luci-app-pushbot/compile` 报错？

LEDE/Lean 等固件**只为 feeds 里的包**生成 `compile`/`install` 目标。若你只是把本包复制到 `package/luci-app-pushbot/`，会报：

```text
No rule to make target 'package/luci-app-pushbot/compile'
```

**解决：** 用下面的**自建 feed（方式 C）**，再编 `package/feeds/pushbot/luci-app-pushbot`。

---

## 方式 C：自建 feed（推荐，Lean/coolsnowwolf LEDE 必用）

coolsnowwolf 的 luci 源在 `feeds update luci` 时**只用远端 git 的索引**，不会把你在本机拷到 `feeds/luci/applications/` 的包加进去，所以会报 “No feed for package 'luci-app-pushbot' found”。必须用自建 feed。

1. 确保包在 `package/` 下（你已做过可跳过）：
   ```bash
   cp -r /path/to/luci-app-pushbot /data/lede/package/
   ```

2. 在 **feeds.conf** 或 **feeds.conf.default** 末尾加一行（路径必须是**包含** `luci-app-pushbot` 的那层目录）：
   ```text
   src-link pushbot /data/lede/package
   ```

3. 更新 feed、安装包、编译：
   ```bash
   cd /data/lede
   ./scripts/feeds update pushbot
   ./scripts/feeds install luci-app-pushbot
   make package/feeds/pushbot/luci-app-pushbot/compile V=s
   ```

4. 生成的 ipk 在 `bin/packages/.../luci-app-pushbot_*.ipk`，按你的 target 路径找即可。

---

## 方式 A：上游 OpenWrt 或自维护 luci 源

仅当 luci 源是你自己维护、且会**根据本地 `feeds/luci/` 生成索引**时，才可以把包拷进 `feeds/luci/applications/`，再 `feeds update luci`、`feeds install luci-app-pushbot`，然后用：
`make package/feeds/luci/luci-app-pushbot/compile V=s`。  
Lean/coolsnowwolf 的 luci 来自远端 git，**不支持**这种方式，请用方式 C。

# 改名公告
#### 2021年04月25日 起luci-app-serverchand 改名为 luci-app-pushbot

如需拉取编译
请把：

`# git clone https://github.com/zzsj0928/luci-app-serverchand package/luci-app-serverchand`

改为

`git clone https://github.com/zzsj0928/luci-app-pushbot package/luci-app-pushbot`

并把 .config 中

`CONFIG_PACKAGE_luci-app-serverchand=y`

改为

`CONFIG_PACKAGE_luci-app-pushbot=y`

注意：本次改名需要提前备份serverchand配置，并于PushBot中重新配置。

再次谢谢各位支持

# 申明
- 本插件由[tty228/luci-app-serverchan](https://github.com/tty228/luci-app-serverchan)原创.
- 因微信推送存在诸多弊端（无法分开聊天工具与功能性消息推送，通知内不显示内容，内容需要点开才能查看等）,
- 故由  然后七年  @zzsj0928 重新修改为本插件，为钉钉机器人API使用。
- 本插件工作在：openwrt
- 本插件支持：钉钉推送,企业微信推送,PushPlus推送,微信推送,企业微信应用推送,飞书推送,钉钉机器人推送,企业微信机器人推送,飞书机器人推送,一对多推送,Bark推送(仅iOS),PushDeer,PushDeer自架
- 自20210911之后的版本，支持Bark群组，群组名默认为设备名
- 自20210901之后的版本，增加依赖jq，请重新编译或在安装前同步安装jq

# 显示效果
## 通知栏：直接显示推送主题，一目了然，按设备不同，分组显示
<img src="https://raw.githubusercontent.com/zzsj0928/ReadmeContents/main/Serverchand/Msg.Notification.jpg" width="500">

## 消息列表：直接显示最新推送的标题
<img src="https://raw.githubusercontent.com/zzsj0928/ReadmeContents/main/Serverchand/Msg.List.jpg" width="500">

## 消息内容：直接显示所有推送信息，不用二次点开再查看
<img src="https://raw.githubusercontent.com/zzsj0928/ReadmeContents/main/Serverchand/MsgContentDetials.jpeg" width="500">

# 下载
- [luci-app-pushbot](https://github.com/zzsj0928/luci-app-pushbot/releases)


-----------------------------------------------------
#####################################################
-----------------------------------------------------

# 以下为原插件简介：

# 简介
- 用于 OpenWRT/LEDE 路由器上进行 Server酱 微信/Telegram 推送的插件
- 基于 serverchan 提供的接口发送信息，Server酱说明：http://sc.ftqq.com/1.version
- **基于斐讯 k3 制作，不同系统不同设备，请自行修改部分代码，无测试条件无法重现的 bug 不考虑修复**
- 依赖 iputils-arping + curl 命令，安装前请 `opkg update`，小内存路由谨慎安装
- 使用主动探测设备连接的方式检测设备在线状态，以避免WiFi休眠机制，主动探测较为耗时，**如遇设备休眠频繁，请自行调整超时设置**
#### 主要功能
- 路由 ip/ipv6 变动推送
- 设备别名
- 设备上线推送
- 设备离线推送
- CPU 负载、温度监视
- 定时推送设备运行状态
- MAC 白名单、黑名单、按接口检测设备
- 免打扰
- 无人值守任务

#### 说明
- 潘多拉系统、或不支持 sh 的系统，请将脚本开头 `#!/bin/sh` 改为 `#!/bin/bash`，或手动安装 `sh`
- 追新是没有意义的，没有问题没必要更新，上班事情忙完了，摸鱼又不会摸，只能靠写几行 bug ，才能缓解无聊这样子

#### 已知问题
- 直接关闭接口时，该接口的离线设备会忽略检测
- 部分设备无法读取到设备名，脚本使用 `cat /var/dhcp.leases` 命令读取设备名，如果 dhcp 中不存在设备名，则无法读取设备名（如二级路由设备、静态ip设备），请使用设备名备注

# Download
- [luci-app-serverchan](https://github.com/tty228/luci-app-serverchan/releases)
- [wrtbwmon](https://github.com/brvphoenix/wrtbwmon)
- [luci-app-wrtbwmon](https://github.com/brvphoenix/luci-app-wrtbwmon) 

#### ps
- 新功能看情况开发
- 王者荣耀新赛季，不思进取中
- 欢迎各种代码提交
- 提交bug时请尽量带上设备信息，日志与描述（如执行`/usr/bin/serverchan/serverchan`后的提示、日志信息、/tmp/serverchan/ipAddress 文件信息）
- 三言两句恕我无能为力
- 武汉加油

