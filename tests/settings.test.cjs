const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const vm=require('node:vm');
const source=fs.readFileSync('web/app.js','utf8');
const context=vm.createContext({});
vm.runInContext(source.slice(0,source.indexOf('function render(')),context);

test('custom delay boundaries color numbers and history consistently without changing score tones',()=>{
 vm.runInContext('state={settings:{delay_green_ms:100,delay_yellow_ms:400}}',context);
 for(const [value,want] of [[100,'good'],[101,'medium'],[400,'medium'],[401,'bad']]){
  assert.equal(vm.runInContext(`delayTone(${value})`,context),want);
  const html=context.history([{at:'2026-10-18T00:00:00Z',success:true,delay_ms:value,score:value}],'delay',100);
  assert.match(html,new RegExp(`bar-${want}`));
 }
 assert.equal(vm.runInContext('tone(130,100)',context),'good');
});

test('changing thresholds invalidates unchanged node rows so cached colors repaint',()=>{
 const node={id:'n',checks:1,score:120,delay_ms:150,history:[]};
 const before=context.nodeKey(node,120,{settings:{delay_green_ms:200,delay_yellow_ms:600}});
 const after=context.nodeKey(node,120,{settings:{delay_green_ms:100,delay_yellow_ms:400}});
 assert.notEqual(before,after);
});

test('LuCI service settings stage JSON with restricted ACL and never invoke a user command',()=>{
 const module=fs.readFileSync('package/luci-app-sing-box-smart/htdocs/luci-static/resources/sing-box-smart/settings.js','utf8');
 const acl=JSON.parse(fs.readFileSync('package/luci-app-sing-box-smart/root/usr/share/rpcd/acl.d/luci-app-sing-box-smart.json','utf8'))['luci-app-sing-box-smart'];
 assert.match(module,/fs\.write\('\/etc\/sing-box-smart\/service\.pending\.json',JSON\.stringify\(settings\),384\)/);
 assert.match(module,/fs\.exec\('\/usr\/bin\/sing-box-smart',\['service-settings','apply'\]\)/);
 assert.deepEqual(acl.write.file['/etc/sing-box-smart/service.pending.json'],['write']);
 assert.equal(acl.write.file['/etc/sing-box-smart/service.json'],undefined);
});
