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

test('IPK extraction can create files without implicitly creating missing directories',()=>{
 const script=`
import sys,io,tarfile,posixpath
sys.path.insert(0,'scripts')
from ipk_archive import archive
names=['www/luci-static/resources/view/sing-box-smart/v0_2_41/app.js','www/luci-static/resources/sing-box-smart/v0_2_41/style.css']
content=archive([(name,b'content',0o644) for name in names])
known={'.'}
with tarfile.open(fileobj=io.BytesIO(content)) as tar:
 for entry in tar:
  parent=posixpath.dirname(entry.name) or '.'
  assert parent in known, (entry.name,'missing parent',parent)
  if entry.isdir(): known.add(entry.name)
  else:
   assert entry.name in names
   assert tar.extractfile(entry).read()==b'content'
print('ok')
`;
 assert.equal(execFileSync('python3',['-c',script],{encoding:'utf8'}).trim(),'ok');
});
