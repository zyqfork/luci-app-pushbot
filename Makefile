#
# LuCI app for Pushbot (Go 版)，参考 Tailscale 等使用 golang-package.mk 构建
#

include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-pushbot
PKG_VERSION:=3.61
PKG_RELEASE:=1

PKG_MAINTAINER:=tty228 <tty228@yeah.net>  zzsj0928

LUCI_TITLE:=LuCI support for Pushbot
LUCI_PKGARCH:=all
# Go 版零依赖；可选安装 iputils-arping 以改善设备检测（否则仅用 ping）
LUCI_DEPENDS:=

# 构建时用 golang-package.mk 编译 pushbot-go，需 feeds 中有 lang/golang（如 openwrt packages）
PKG_BUILD_DEPENDS:=golang/host
PKG_BUILD_PARALLEL:=1

# Go 模块路径（与 pushbot-go/go.mod 中 module 一致）
GO_PKG:=github.com/zzsj0928/pushbot-go

define Package/$(PKG_NAME)/conffiles
/etc/config/pushbot
/usr/bin/pushbot/api/diy.json
/usr/bin/pushbot/api/ipv4.list
/usr/bin/pushbot/api/ipv6.list
endef

# 包目录下手动生成二进制（无 golang feed 时）: make -f Makefile.prepare prepare
prepare:
	cd pushbot-go && GOOS=linux go build -o pushbot . && mkdir -p ../root/usr/bin/pushbot && cp pushbot ../root/usr/bin/pushbot/pushbot

# 先 luci.mk（提供 LuCI 的 Prepare/Install），再 golang-package.mk（覆盖 Build/Compile 为 Go 编译）
include $(TOPDIR)/feeds/luci/luci.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

# 覆盖 Prepare：先执行 LuCI 默认（复制 luasrc/root/ 等），再把 pushbot-go 铺到 PKG_BUILD_DIR 供 go install
define Build/Prepare
	for d in luasrc ucode htdocs root src; do \
		if [ -d ./$$d ]; then \
			mkdir -p $(PKG_BUILD_DIR)/$$d; \
			$(CP) ./$$d/* $(PKG_BUILD_DIR)/$$d/; \
		fi; \
	done
	$(call Build/Prepare/$(LUCI_NAME))
	$(call Build/Prepare/Default)
	cp -r $(CURDIR)/pushbot-go/* $(PKG_BUILD_DIR)/
endef

# Go 编译完成后，将二进制放入 root/usr/bin/pushbot/pushbot（与 LuCI 的 root/ 一起被打包）
define Build/Compile
	$(call GoPackage/Build/Compile)
	mkdir -p $(PKG_BUILD_DIR)/root/usr/bin/pushbot
	cp $(GO_PKG_BUILD_BIN_DIR)/pushbot-go $(PKG_BUILD_DIR)/root/usr/bin/pushbot/pushbot
endef
