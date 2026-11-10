const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

const acl = JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/rpcd/acl.d/luci-app-sing-box-smart.json', 'utf8'))['luci-app-sing-box-smart'];
const view = fs.readFileSync('package/luci-app-sing-box-smart/htdocs/luci-static/resources/view/sing-box-smart/app.js', 'utf8');
const init = fs.readFileSync('deploy/sing-box-smart.init', 'utf8');
const corePackage = fs.readFileSync('package/sing-box-smart/Makefile', 'utf8');
const luciPackage = fs.readFileSync('package/luci-app-sing-box-smart/Makefile', 'utf8');
const menu = JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/luci/menu.d/luci-app-sing-box-smart.json', 'utf8'));
const feed = fs.readFileSync('feed.sh', 'utf8');
const install = fs.readFileSync('install.sh', 'utf8');
const uninstall = fs.readFileSync('uninstall.sh', 'utf8');

test('LuCI ACL is least privilege and exposes only fixed init actions', () => {
 assert.deepEqual(acl.read.ubus, {service:['list']});
 assert.deepEqual(Object.keys(acl.read.file).sort(), ['/etc/init.d/sing-box-smart enabled','/usr/bin/sing-box-smart config-settings check','/usr/bin/sing-box-smart config-settings read'].sort());
 assert.deepEqual(Object.keys(acl.write.file).sort(), ['disable','enable','restart','start','stop'].map(a => `/etc/init.d/sing-box-smart ${a}`).concat(['/usr/bin/sing-box-smart config-settings apply','/etc/sing-box-smart/config.pending.json']).sort());
 assert.equal('ubus' in acl.write, false);
 assert.deepEqual(acl.write.file['/etc/sing-box-smart/config.pending.json'],['write']);
});

test('LuCI view cannot execute user supplied commands and serializes controls', () => {
 assert.match(view, /\['start','stop','restart'\]/);
 assert.match(view, /'enable' : 'disable'/);
 assert.match(view, /if \(busy\) return Promise\.resolve\(\)/);
 assert.match(view, /service && service\.instances/);
 assert.doesNotMatch(view, /fs\.exec\([^,]+,\s*\[[^\]]*target/);
 assert.match(view, /sbs-auto-row/);
 assert.match(view, /监控面板/);
 assert.match(view, /config-settings/);
 assert.doesNotMatch(view, /sing-box-smart-update/);
 assert.doesNotMatch(view, /fs\.exec\([^,]+,\s*\[[^\]]*https?:/);
 assert.doesNotMatch(view, /sbs-link-card|sbs-arrow|打开 smart 面板.*↗/);
});

test('LuCI menu and init script point at the packaged service', () => {
 assert.equal(menu['admin/services/sing-box-smart'].action.path, 'sing-box-smart/app');
 assert.match(init, /USE_PROCD=1/);
 assert.match(init, /\/usr\/bin\/sing-box-smart/);
 assert.match(init, /-settings \/etc\/sing-box-smart\/connection\.json/);
 assert.match(init, /procd_set_param respawn/);
});

test('LuCI updates preserve the active WRT login session', () => {
 assert.doesNotMatch(corePackage, /rpcd restart/);
});

test('APK lifecycle starts clean installs and purges runtime state on removal', () => {
 assert.match(corePackage, /Package\/sing-box-smart\/postinst/);
 assert.match(corePackage, /Package\/sing-box-smart\/prerm/);
 assert.match(corePackage, /Package\/sing-box-smart\/postrm/);
 assert.match(corePackage, /rm -rf \/etc\/sing-box-smart \/var\/log\/sing-box-smart/);
 assert.match(luciPackage, /Package\/luci-app-sing-box-smart\/postinst/);
 assert.match(luciPackage, /Package\/luci-app-sing-box-smart\/postrm/);
});

test('Feed is the only supported installation path', () => {
 assert.match(feed, /25\.12/);
 assert.match(feed, /aarch64_generic/);
 assert.match(feed, /\/etc\/apk\/repositories\.d/);
 assert.match(feed, /customfeeds\.list/);
 assert.match(feed, /packages\.adb/);
 assert.match(install, /feed\.sh/);
 assert.match(install, /apk add sing-box-smart luci-app-sing-box-smart/);
 assert.match(uninstall, /apk del --purge luci-app-sing-box-smart sing-box-smart/);
 assert.match(install, /sing-box-smart\.pages\.dev\/feed\.sh/);
 assert.doesNotMatch(install, /opkg|allow-untrusted|github\.com\/.*\/releases|raw\.githubusercontent\.com/);
 assert.doesNotMatch(uninstall, /opkg|allow-untrusted/);
});
