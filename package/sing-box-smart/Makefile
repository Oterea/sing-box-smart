include $(TOPDIR)/rules.mk
PKG_NAME:=sing-box-smart
PKG_VERSION?=0.2.0
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_BUILD_DEPENDS:=golang/host
GO_PKG:=sing-box-smart
GO_PKG_BUILD_PKG:=sing-box-smart/cmd/sing-box-smart
include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Build/Prepare
	mkdir -p $(PKG_BUILD_DIR)
	cp -a /feed/. $(PKG_BUILD_DIR)/
endef

define Package/sing-box-smart
 SECTION:=net
 CATEGORY:=Network
 TITLE:=sing-box-smart monitor
 DEPENDS:=$(GO_ARCH_DEPENDS)
endef

define Package/sing-box-smart/conffiles
/etc/sing-box-smart/connection.json
/etc/sing-box-smart/runtime.json
/etc/sing-box-smart/service.json
endef

define Package/sing-box-smart/install
	$(call GoPackage/Package/Install/Bin,$(1))
	$(INSTALL_DIR) $(1)/etc/init.d
	$(INSTALL_BIN) $(CURDIR)/../../deploy/sing-box-smart.init $(1)/etc/init.d/sing-box-smart
	$(INSTALL_DIR) $(1)/etc/sing-box-smart
	$(INSTALL_CONF) $(CURDIR)/../../deploy/defaults/connection.json $(1)/etc/sing-box-smart/connection.json
	$(INSTALL_CONF) $(CURDIR)/../../deploy/defaults/runtime.json $(1)/etc/sing-box-smart/runtime.json
	$(INSTALL_CONF) $(CURDIR)/../../deploy/defaults/service.json $(1)/etc/sing-box-smart/service.json
	$(INSTALL_DIR) $(1)/lib/upgrade/keep.d
	$(INSTALL_DATA) $(CURDIR)/../../deploy/sing-box-smart.keep $(1)/lib/upgrade/keep.d/sing-box-smart
endef

define Package/sing-box-smart/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] && exit 0
/etc/init.d/sing-box-smart enable
/etc/init.d/sing-box-smart restart || /etc/init.d/sing-box-smart start
exit 0
endef

define Package/sing-box-smart/prerm
#!/bin/sh
[ -x /etc/init.d/sing-box-smart ] && /etc/init.d/sing-box-smart stop || :
exit 0
endef

define Package/sing-box-smart/postrm
#!/bin/sh
case "$${1:-}" in
remove|purge)
	rm -rf /etc/sing-box-smart /var/log/sing-box-smart
	rm -rf /tmp/sing-box-smart-update /tmp/sing-box-smart-update.lock
	rm -f /tmp/sing-box-smart-update.status /tmp/sing-box-smart-update.log
	rm -f /lib/upgrade/keep.d/sing-box-smart
	;;
esac
exit 0
endef

$(eval $(call GoBinPackage,sing-box-smart))
$(eval $(call BuildPackage,sing-box-smart))
