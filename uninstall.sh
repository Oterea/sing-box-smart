#!/bin/sh
set -eu

apk del --purge luci-app-sing-box-smart sing-box-smart || true
rm -f \
    /www/luci-static/resources/sing-box-smart/coreconfig.js \
    /www/luci-static/resources/sing-box-smart/style.css \
    /www/luci-static/resources/view/sing-box-smart/app.js
for base in /www/luci-static/resources/sing-box-smart /www/luci-static/resources/view/sing-box-smart; do
    for directory in "$base"/v[0-9]*; do
        [ -d "$directory" ] && rmdir "$directory" 2>/dev/null || :
    done
done
rm -f /tmp/luci-indexcache /tmp/luci-indexcache.* /tmp/luci-modulecache/*
if [ -f /etc/apk/repositories.d/customfeeds.list ]; then
    sed -i '/sing-box-smart\.pages\.dev/d' /etc/apk/repositories.d/customfeeds.list
fi
rm -f /etc/apk/keys/sing-box-smart.pem
apk update
