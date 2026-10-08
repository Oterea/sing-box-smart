const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

const acl = JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/rpcd/acl.d/luci-app-sing-box-smart.json', 'utf8'))['luci-app-sing-box-smart'];
const view = fs.readFileSync('package/luci-app-sing-box-smart/htdocs/luci-static/resources/view/sing-box-smart/app.js', 'utf8');
const updateStatus = fs.readFileSync('package/luci-app-sing-box-smart/root/usr/libexec/sing-box-smart-update-status', 'utf8');
const init = fs.readFileSync('deploy/sing-box-smart.init', 'utf8');
const packer = fs.readFileSync('scripts/build-ipk.py', 'utf8');
const menu = JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/luci/menu.d/luci-app-sing-box-smart.json', 'utf8'));

test('LuCI ACL is least privilege and exposes only fixed init actions', () => {
 assert.deepEqual(acl.read.ubus, {service:['list']});
 assert.deepEqual(Object.keys(acl.read.file).sort(), ['/etc/init.d/sing-box-smart enabled','/usr/libexec/sing-box-smart-update-check','/usr/libexec/sing-box-smart-update-status'].sort());
 assert.deepEqual(Object.keys(acl.write.file).sort(), ['disable','enable','restart','start','stop'].map(a => `/etc/init.d/sing-box-smart ${a}`).concat(['/usr/libexec/sing-box-smart-update-start']).sort());
 assert.equal('ubus' in acl.write, false);
});

test('LuCI view cannot execute user supplied commands and serializes controls', () => {
 assert.match(view, /\['start','stop','restart'\]/);
 assert.match(view, /'enable' : 'disable'/);
 assert.match(view, /if \(busy\) return Promise\.resolve\(\)/);
 assert.match(view, /service && service\.instances/);
 assert.doesNotMatch(view, /fs\.exec\([^,]+,\s*\[[^\]]*target/);
 assert.match(view, /sbs-auto-row/);
 assert.match(view, /打开监控面板/);
 assert.match(view, /sing-box-smart-update-check/);
 assert.match(view, /sing-box-smart-update-start/);
 assert.match(view, /sing-box-smart-update-status/);
 assert.match(view, /progress\+'%'/);
 assert.match(updateStatus, /state.*success/);
 assert.match(updateStatus, /rm -f.*\$status/);
 assert.match(view, /已是最新版本 ·/);
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

test('IPK builder uses the target opkg tar.gz container', () => {
 assert.match(packer, /def write_ipk\(/);
 assert.match(packer, /tarfile\.open\(path, mode='w:gz'\)/);
 assert.match(packer, /write_ipk\(path/);
 assert.doesNotMatch(packer, /\['ar',\s*'r'/);
});
