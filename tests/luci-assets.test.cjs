const {test}=require('node:test');
const assert=require('node:assert/strict');
const {execFileSync}=require('node:child_process');

test('LuCI packages use separate URLs for each release across menu, view, settings and CSS',()=>{
 const script=`
import sys,json
from pathlib import Path
sys.path.insert(0,'scripts')
from luci_assets import collect_luci_files
root=Path('package/luci-app-sing-box-smart')
result=[]
for version in ['0.2.39','0.2.40']:
 files={name:data.decode() for name,data,mode in collect_luci_files(root,version)}
 menu=json.loads(files['usr/share/luci/menu.d/luci-app-sing-box-smart.json'])
 view=menu['admin/services/sing-box-smart']['action']['path']
 app=files['www/luci-static/resources/view/'+view+'.js']
 result.append({'view':view,'names':list(files),'app':app})
print(json.dumps(result))
`;
 const [old,current]=JSON.parse(execFileSync('python3',['-c',script],{encoding:'utf8'}));
 assert.notEqual(old.view,current.view);
 assert.equal(current.view,'sing-box-smart/v0_2_40/app');
 assert.match(current.app,/require sing-box-smart\.v0_2_40\.settings as serviceSettings/);
 assert.match(current.app,/L\.resource\('sing-box-smart\/v0_2_40\/style.css'\)/);
 assert.ok(current.names.includes('www/luci-static/resources/sing-box-smart/v0_2_40/settings.js'));
 assert.ok(current.names.includes('www/luci-static/resources/sing-box-smart/v0_2_40/style.css'));
 assert.ok(current.names.filter(n=>n.startsWith('www/')).every(n=>!old.names.includes(n)));
});
