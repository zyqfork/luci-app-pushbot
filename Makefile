#
# LuCI app for Pushbot（C 版，无 Go 依赖）
#

include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-pushbot
PKG_VERSION:=3.61
PKG_RELEASE:=1

PKG_MAINTAINER:=tty228 <tty228@yeah.net>  zzsj0928

LUCI_TITLE:=LuCI support for Pushbot
LUCI_PKGARCH:=all
LUCI_DEPENDS:=

PKG_BUILD_PARALLEL:=1

define Package/$(PKG_NAME)/conffiles
/etc/config/pushbot
/usr/bin/pushbot/api/diy.json
/usr/bin/pushbot/api/ipv4.list
/usr/bin/pushbot/api/ipv6.list
endef

LUCI_MK:=$(firstword $(wildcard $(TOPDIR)/feeds/luci/luci.mk $(TOPDIR)/package/feeds/luci/luci.mk))
ifneq ($(LUCI_MK),)
  include $(LUCI_MK)
else
  $(error luci.mk not found. Run: ./scripts/feeds update luci)
endif

# C 编译：无 Go 依赖，仅需 TARGET_CC
define Build/Compile
	$(MAKE) -C $(PKG_BUILD_DIR)/pushbot-c \
		CC="$(TARGET_CC)" \
		CFLAGS="$(TARGET_CFLAGS)" \
		LDFLAGS="$(TARGET_LDFLAGS)" \
		clean all
	mkdir -p $(PKG_BUILD_DIR)/root/usr/bin/pushbot
	cp $(PKG_BUILD_DIR)/pushbot-c/pushbot $(PKG_BUILD_DIR)/root/usr/bin/pushbot/pushbot
endef

define Build/Prepare
	for d in luasrc ucode htdocs root src; do \
		if [ -d ./$$d ]; then \
			mkdir -p $(PKG_BUILD_DIR)/$$d; \
			$(CP) ./$$d/* $(PKG_BUILD_DIR)/$$d/; \
		fi; \
	done
	$(call Build/Prepare/$(LUCI_NAME))
	$(call Build/Prepare/Default)
	cp -r $(CURDIR)/pushbot-c $(PKG_BUILD_DIR)/
endef

# 包目录下手动编译 C 二进制: make -f Makefile.prepare prepare
prepare:
	$(MAKE) -C pushbot-c clean all
	mkdir -p root/usr/bin/pushbot
	cp pushbot-c/pushbot root/usr/bin/pushbot/pushbot
