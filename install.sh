#!/bin/sh
set -eu

feed_url='https://sing-box-smart.pages.dev/feed.sh'
tmp="/tmp/sing-box-smart-feed.$$"
trap 'rm -f "$tmp"' EXIT
wget -q -O "$tmp" "$feed_url"
sh "$tmp"
apk add sing-box-smart luci-app-sing-box-smart
