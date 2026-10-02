include $(TOPDIR)/rules.mk
PKG_NAME:=sing-box-smart
PKG_VERSION:=0.2.0
PKG_RELEASE:=1
PKG_LICENSE:=MIT
PKG_BUILD_DEPENDS:=golang/host
GO_PKG:=sing-box-smart
GO_PKG_BUILD_PKG:=sing-box-smart/cmd/sing-box-smart
include $(INCLUDE_DIR)/package.mk
include $(TOPDIR)/feeds/packages/lang/golang/golang-package.mk

define Package/sing-box-smart
 SECTION:=net
 CATEGORY:=Network
 TITLE:=sing-box-smart monitor
 DEPENDS:=$(GO_ARCH_DEPENDS)
endef

define Package/sing-box-smart/conffiles
/etc/sing-box-smart.conf
endef

define Package/sing-box-smart/install
 $(call GoPackage/Package/Install/Bin,$(1))
 $(INSTALL_DIR) $(1)/etc/init.d
 $(INSTALL_BIN) $(CURDIR)/../../deploy/sing-box-smart.init $(1)/etc/init.d/sing-box-smart
 $(INSTALL_CONF) $(CURDIR)/../../deploy/sing-box-smart.conf $(1)/etc/sing-box-smart.conf
 $(INSTALL_DIR) $(1)/lib/upgrade/keep.d
 $(INSTALL_DATA) $(CURDIR)/../../deploy/sing-box-smart.keep $(1)/lib/upgrade/keep.d/sing-box-smart
endef
$(eval $(call GoBinPackage,sing-box-smart))
$(eval $(call BuildPackage,sing-box-smart))
