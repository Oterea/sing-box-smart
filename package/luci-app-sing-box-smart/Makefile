include $(TOPDIR)/rules.mk
PKG_VERSION?=0.2.0
LUCI_TITLE:=LuCI support for sing-box-smart
LUCI_DEPENDS:=+luci-base +sing-box-smart +uclient-fetch +jsonfilter

define Package/luci-app-sing-box-smart/postinst
#!/bin/sh
[ -n "$${IPKG_INSTROOT}" ] && exit 0
/usr/libexec/sing-box-smart-cleanup-luci
rm -f /tmp/luci-indexcache /tmp/luci-indexcache.* /tmp/luci-modulecache/*
exit 0
endef

define Package/luci-app-sing-box-smart/postrm
#!/bin/sh
case "$${1:-}" in
remove|purge)
	rm -f /tmp/luci-indexcache /tmp/luci-indexcache.* /tmp/luci-modulecache/*
	;;
esac
exit 0
endef

include $(TOPDIR)/feeds/luci/luci.mk

# call BuildPackage - OpenWrt buildroot signature
