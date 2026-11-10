#!/bin/sh
set -eu

if [ ! -x /usr/bin/apk ]; then
    echo '此 Feed 只支持 OpenWrt 25.12 APK 系统。' >&2
    exit 1
fi

. /etc/openwrt_release
case "${DISTRIB_RELEASE:-}" in
    *25.12*) ;;
    *) echo "不支持的 OpenWrt 版本：${DISTRIB_RELEASE:-unknown}" >&2; exit 1 ;;
esac

arch="${DISTRIB_ARCH:-$(cat /etc/apk/arch 2>/dev/null || true)}"
[ "$arch" = 'aarch64_generic' ] || { echo "不支持的架构：$arch" >&2; exit 1; }

repository_url='https://sing-box-smart.pages.dev'
feed_url="$repository_url/openwrt-25.12/$arch"
key='/etc/apk/keys/sing-box-smart.pem'
repo='/etc/apk/repositories.d/sing-box-smart.list'
mkdir -p "$(dirname "$key")" "$(dirname "$repo")"

tmp="${key}.tmp.$$"
trap 'rm -f "$tmp"' EXIT
wget -q -O "$tmp" "$repository_url/public-key.pem"
chmod 0644 "$tmp"
mv "$tmp" "$key"

touch "$repo"
sed -i '/sing-box-smart\.pages\.dev/d' "$repo"
echo "$feed_url/packages.adb" >> "$repo"
apk update
echo 'sing-box-smart Feed 配置完成。'
