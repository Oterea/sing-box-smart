#!/bin/sh
set -eu

apk del --purge luci-app-sing-box-smart sing-box-smart || true
rm -f /etc/apk/repositories.d/sing-box-smart.list
rm -f /etc/apk/keys/sing-box-smart.pem
apk update
