const {test}=require('node:test');
const assert=require('node:assert/strict');
const fs=require('node:fs');
const os=require('node:os');
const path=require('node:path');
const {promisify}=require('node:util');
const execFile=promisify(require('node:child_process').execFile);
const root='package/luci-app-sing-box-smart/root/usr/libexec/';

test('update checks bypass stale manifests and isolate concurrent temporary files',async()=>{
 const dir=fs.mkdtempSync(path.join(os.tmpdir(),'smart-cache-test-'));
 const mock=(name,source)=>fs.writeFileSync(path.join(dir,name),'#!'+process.execPath+'\n'+source,{mode:0o755});
 try {
  mock('uclient-fetch',`const fs=require('node:fs');const a=process.argv.slice(2);const dest=a[a.indexOf('-O')+1];const fresh=/[?]check=\\d+-\\d+$/.test(a.at(-1));fs.appendFileSync(process.env.FETCH_LOG,dest+'\\n');fs.writeFileSync(dest,JSON.stringify({version:fresh?'0.2.38':'0.2.37'}));`);
  mock('jsonfilter',`const fs=require('node:fs');const a=process.argv.slice(2);console.log(JSON.parse(fs.readFileSync(a[a.indexOf('-i')+1])).version);`);
  mock('opkg',`console.log('Version: 0.2.37');`);
  const log=path.join(dir,'fetch.log');
  const options={env:{...process.env,PATH:dir+':'+process.env.PATH,FETCH_LOG:log}};
  const results=await Promise.all([execFile('/bin/sh',[root+'sing-box-smart-update-check'],options),execFile('/bin/sh',[root+'sing-box-smart-update-check'],options)]);
  for(const r of results)assert.deepEqual(JSON.parse(r.stdout),{available:true,version:'0.2.38',current:'0.2.37'});
  const files=fs.readFileSync(log,'utf8').trim().split('\n');
  assert.equal(new Set(files).size,2);
  for(const file of files)assert.equal(fs.existsSync(file),false,'temporary manifest must be removed');
 }finally{fs.rmSync(dir,{recursive:true,force:true});}
});
