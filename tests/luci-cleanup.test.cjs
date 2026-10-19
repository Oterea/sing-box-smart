const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const {execFileSync}=require('node:child_process');
const cleanup='package/luci-app-sing-box-smart/root/usr/libexec/sing-box-smart-cleanup-luci';

test('LuCI cleanup removes only empty version directories and can run repeatedly',()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'smart-luci-cleanup-'));
 try{
  const bases=['sing-box-smart','view/sing-box-smart'].map(name=>path.join(root,'www/luci-static/resources',name));
  for(const base of bases){
   for(const name of ['v0_2_40','v0_2_63','v0_2_64','custom','v0_2_41/nested'])fs.mkdirSync(path.join(base,name),{recursive:true});
   fs.writeFileSync(path.join(base,'v0_2_63','old.js'),'old resource');
   fs.writeFileSync(path.join(base,'v0_2_64','current.js'),'current resource');
   fs.symlinkSync(path.join(base,'custom'),path.join(base,'v0_2_42'));
  }
  const config=path.join(root,'etc/sing-box-smart');fs.mkdirSync(config,{recursive:true});
  fs.writeFileSync(path.join(config,'runtime.json'),'saved settings');
  for(let run=0;run<2;run++)execFileSync('/bin/sh',[cleanup],{env:{...process.env,IPKG_INSTROOT:root}});
  for(const base of bases){
   assert.equal(fs.existsSync(path.join(base,'v0_2_40')),false);
   assert.equal(fs.readFileSync(path.join(base,'v0_2_63','old.js'),'utf8'),'old resource');
   assert.equal(fs.readFileSync(path.join(base,'v0_2_64','current.js'),'utf8'),'current resource');
   assert.equal(fs.existsSync(path.join(base,'custom')),true);
   assert.equal(fs.existsSync(path.join(base,'v0_2_41/nested')),true);
   assert.equal(fs.lstatSync(path.join(base,'v0_2_42')).isSymbolicLink(),true);
  }
  assert.equal(fs.readFileSync(path.join(config,'runtime.json'),'utf8'),'saved settings');
 }finally{fs.rmSync(root,{recursive:true,force:true});}
});

test('LuCI cleanup accepts an installation without any existing resource directories',()=>{
 const root=fs.mkdtempSync(path.join(os.tmpdir(),'smart-luci-cleanup-empty-'));
 try{execFileSync('/bin/sh',[cleanup],{env:{...process.env,IPKG_INSTROOT:root}});assert.deepEqual(fs.readdirSync(root),[]);}
 finally{fs.rmSync(root,{recursive:true,force:true});}
});
