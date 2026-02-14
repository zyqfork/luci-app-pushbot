#
# LuCI app for Pushbot (Go 版)
# 使用说明：需将本包放入 LuCI 源内并执行 feeds update/install，或在有 feeds 的 LEDE 中放入 package/feeds/luci/
#

include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-pushbot
PKG_VERSION:=3.61
PKG_RELEASE:=1

PKG_MAINTAINER:=tty228 <tty228@yeah.net>  zzsj0928

LUCI_TITLE:=LuCI support for Pushbot
LUCI_PKGARCH:=all
LUCI_DEPENDS:=

# 有 golang feed 时再追加 golang/host，否则不依赖
PKG_BUILD_PARALLEL:=1
GO_PKG:=github.com/zzsj0928/pushbot-go

define Package/$(PKG_NAME)/conffiles
/etc/config/pushbot
/usr/bin/pushbot/api/diy.json
/usr/bin/pushbot/api/ipv4.list
/usr/bin/pushbot/api/ipv6.list
endef

# 兼容不同 LEDE/OpenWrt 源布局：优先 feeds/luci，其次 package/feeds/luci
LUCI_MK:=$(firstword $(wildcard $(TOPDIR)/feeds/luci/luci.mk $(TOPDIR)/package/feeds/luci/luci.mk))
ifneq ($(LUCI_MK),)
  include $(LUCI_MK)
else
  $(error luci.mk not found. Run: ./scripts/feeds update luci && ./scripts/feeds install -a)
endif

# 有 golang-package.mk 时用其编译 Go，否则仅复制 root（需事先 make -f Makefile.prepare prepare）
GOLANG_MK:=$(firstword $(wildcard $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk $(TOPDIR)/package/feeds/packages/lang/golang/golang-package.mk))
ifneq ($(GOLANG_MK),)
  PKG_BUILD_DEPENDS+=golang/host
  include $(GOLANG_MK)
  define Build/Compile
	$(call GoPackage/Build/Compile)
	mkdir -p $(PKG_BUILD_DIR)/root/usr/bin/pushbot
	cp $(GO_PKG_BUILD_BIN_DIR)/pushbot-go $(PKG_BUILD_DIR)/root/usr/bin/pushbot/pushbot
  endef
else
  define Build/Compile
	@echo "golang-package.mk not found; ensure root/usr/bin/pushbot/pushbot exists (run: make -f Makefile.prepare prepare)"
	@[ -f $(CURDIR)/root/usr/bin/pushbot/pushbot ] || (echo "ERROR: missing root/usr/bin/pushbot/pushbot" && exit 1)
	mkdir -p $(PKG_BUILD_DIR)/root/usr/bin/pushbot
	cp $(CURDIR)/root/usr/bin/pushbot/pushbot $(PKG_BUILD_DIR)/root/usr/bin/pushbot/pushbot
  endef
endif

define Build/Prepare
	for d in luasrc ucode htdocs root src; do \
		if [ -d ./$$d ]; then \
			mkdir -p $(PKG_BUILD_DIR)/$$d; \
			$(CP) ./$$d/* $(PKG_BUILD_DIR)/$$d/; \
		fi; \
	done
	$(call Build/Prepare/$(LUCI_NAME))
	$(call Build/Prepare/Default)
	$(if $(GOLANG_MK),cp -r $(CURDIR)/pushbot-go/* $(PKG_BUILD_DIR)/)
endef

# 包目录下手动生成二进制: make -f Makefile.prepare prepare
prepare:
	cd pushbot-go && GOOS=linux go build -o pushbot . && mkdir -p ../root/usr/bin/pushbot && cp pushbot ../root/usr/bin/pushbot/pushbot
