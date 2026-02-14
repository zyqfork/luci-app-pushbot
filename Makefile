include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-pushbot
PKG_VERSION:=3.61
PKG_RELEASE:=1

PKG_MAINTAINER:=tty228 <tty228@yeah.net>  zzsj0928

LUCI_TITLE:=LuCI support for Pushbot
LUCI_PKGARCH:=all
# Go 版零依赖；可选安装 iputils-arping 以改善设备检测（否则仅用 ping）
LUCI_DEPENDS:=

define Package/$(PKG_NAME)/conffiles
/etc/config/pushbot
/usr/bin/pushbot/api/diy.json
/usr/bin/pushbot/api/ipv4.list
/usr/bin/pushbot/api/ipv6.list
endef

# 编译前生成 Go 二进制：make prepare（需安装 Go），再将 root 打包
prepare:
	cd pushbot-go && GOOS=linux go build -o pushbot . && mkdir -p ../root/usr/bin/pushbot && cp pushbot ../root/usr/bin/pushbot/pushbot

include $(TOPDIR)/feeds/luci/luci.mk

# call BuildPackage - OpenWrt buildroot signature
