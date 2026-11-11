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
	root="$${IPKG_INSTROOT:-}"
	resource_root="$$root/www/luci-static/resources"
	rm -f \
		"$$resource_root/sing-box-smart/coreconfig.js" \
		"$$resource_root/sing-box-smart/style.css" \
		"$$root/www/luci-static/resources/view/sing-box-smart/app.js"
	for base in "$$resource_root/sing-box-smart" "$$resource_root/view/sing-box-smart"; do \
		for directory in "$$base"/v[0-9]*; do \
			[ -d "$$directory" ] && rmdir "$$directory" 2>/dev/null || :; \
		done; \
	done
	rm -f /tmp/luci-indexcache /tmp/luci-indexcache.* /tmp/luci-modulecache/*
	;;
esac
exit 0
endef

include $(TOPDIR)/feeds/luci/luci.mk

# call BuildPackage - OpenWrt buildroot signature
