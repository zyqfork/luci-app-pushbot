include $(TOPDIR)/rules.mk

PKG_NAME:=luci-app-pushbot
PKG_VERSION:=3.61
PKG_RELEASE:=1

PKG_LICENSE:=GPL-3.0
PKG_LICENSE_FILES:=LICENSE
PKG_MAINTAINER:=tty228 <tty228@yeah.net> zzsj0928

include $(INCLUDE_DIR)/package.mk

define Package/$(PKG_NAME)
  SECTION:=luci
  CATEGORY:=LuCI
  SUBMENU:=3. Applications
  TITLE:=LuCI support for Pushbot
  DEPENDS:=+luci-base +libuci
endef

define Package/$(PKG_NAME)/description
  LuCI support for Pushbot (钉钉/企业微信等推送).
endef

define Package/$(PKG_NAME)/conffiles
/etc/config/pushbot
/usr/bin/pushbot/api/diy.json
/usr/bin/pushbot/api/ipv4.list
/usr/bin/pushbot/api/ipv6.list
endef

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	$(CP) ./pushbot-c $(PKG_BUILD_DIR)/
	$(CP) ./luasrc $(PKG_BUILD_DIR)/
	$(CP) ./root $(PKG_BUILD_DIR)/
endef

define Build/Compile
	$(MAKE) -C $(PKG_BUILD_DIR)/pushbot-c \
		CC="$(TARGET_CC)" \
		CFLAGS="$(TARGET_CFLAGS)" \
		LDFLAGS="$(TARGET_LDFLAGS)" \
		clean all
endef

define Package/$(PKG_NAME)/install
	$(INSTALL_DIR) $(1)/usr/lib/lua/luci
	$(CP) $(PKG_BUILD_DIR)/luasrc/* $(1)/usr/lib/lua/luci/
	
	$(INSTALL_DIR) $(1)/usr/bin/pushbot
	$(INSTALL_BIN) $(PKG_BUILD_DIR)/pushbot-c/pushbot $(1)/usr/bin/pushbot/pushbot
	
	$(CP) $(PKG_BUILD_DIR)/root/* $(1)/
endef

$(eval $(call BuildPackage,$(PKG_NAME)))