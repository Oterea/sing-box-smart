const {test} = require('node:test');
const assert = require('node:assert/strict');
const fs = require('node:fs');

const acl = JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/rpcd/acl.d/luci-app-sing-box-smart.json', 'utf8'))['luci-app-sing-box-smart'];
const view = fs.readFileSync('package/luci-app-sing-box-smart/htdocs/luci-static/resources/view/sing-box-smart/app.js', 'utf8');
const init = fs.readFileSync('deploy/sing-box-smart.init', 'utf8');
const menu = JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/luci/menu.d/luci-app-sing-box-smart.json', 'utf8'));

test('LuCI ACL is least privilege and exposes only fixed init actions', () => {
 assert.deepEqual(acl.read.ubus, {service:['list']});
 assert.deepEqual(Object.keys(acl.read.file), ['/etc/init.d/sing-box-smart enabled']);
 assert.deepEqual(Object.keys(acl.write.file).sort(), ['disable','enable','restart','start','stop'].map(a => `/etc/init.d/sing-box-smart ${a}`).sort());
 assert.equal('ubus' in acl.write, false);
});

test('LuCI view cannot execute user supplied commands and serializes controls', () => {
 assert.match(view, /\['start','stop','restart'\]/);
 assert.match(view, /'enable' : 'disable'/);
 assert.match(view, /if \(busy\) return Promise\.resolve\(\)/);
 assert.match(view, /service && service\.instances/);
 assert.doesNotMatch(view, /fs\.exec\([^,]+,\s*\[[^\]]*target/);
});

test('LuCI menu and init script point at the packaged service', () => {
 assert.equal(menu['admin/services/sing-box-smart'].action.path, 'sing-box-smart/app');
 assert.match(init, /USE_PROCD=1/);
 assert.match(init, /\/usr\/bin\/sing-box-smart/);
 assert.match(init, /-settings \/etc\/sing-box-smart\/connection\.json/);
 assert.match(init, /procd_set_param respawn/);
});
