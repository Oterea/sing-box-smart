#!/bin/sh
set -e

cp -f /usr/bin/sbwatch /usr/bin/sbwatch.backup
cp -f /etc/init.d/sbwatch /etc/init.d/sbwatch.backup
/etc/init.d/sbwatch stop
/etc/init.d/sbwatch disable

cp -f /tmp/sing-box-smart-arm64-v2 /usr/bin/sing-box-smart
chmod 755 /usr/bin/sing-box-smart
cp -f /tmp/sing-box-smart.conf /etc/sing-box-smart.conf
chmod 600 /etc/sing-box-smart.conf
cp -f /tmp/sing-box-smart.init /etc/init.d/sing-box-smart
chmod 755 /etc/init.d/sing-box-smart

/etc/init.d/sing-box-smart enable
/etc/init.d/sing-box-smart start
sleep 3
ps w | grep sing-box-smart | grep -v grep
wget -qO- http://127.0.0.1:9797/api/state | head -c 2000
echo
