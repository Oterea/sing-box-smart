#!/bin/sh
set -eu

apk del --purge luci-app-sing-box-smart sing-box-smart || true
if [ -f /etc/apk/repositories.d/customfeeds.list ]; then
    sed -i '/sing-box-smart\.pages\.dev/d' /etc/apk/repositories.d/customfeeds.list
fi
rm -f /etc/apk/keys/sing-box-smart.pem
apk update
