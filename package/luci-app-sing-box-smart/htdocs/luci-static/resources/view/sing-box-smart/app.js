'use strict';
'require view';
'require rpc';
'require fs';
'require poll';
'require ui';
'require sing-box-smart.coreconfig as coreConfig';
const serviceName = 'sing-box-smart';
const initPath = '/etc/init.d/' + serviceName;
const updatePath = '/usr/libexec/sing-box-smart-update';
const list = rpc.declare({object:'service',method:'list',params:['name'],expect:{'':{}}});
return view.extend({
 load: function() { return Promise.all([list(serviceName), fs.exec(initPath,['enabled']), fs.exec('/usr/bin/sing-box-smart',['config-settings','read']), fs.exec(updatePath,['version'])]); },
 render: function(data) {
  // Packaging versions the view, settings module and CSS together.
  const styleURL = L.resource('sing-box-smart/style.css');
  const style = document.querySelector('link[data-sing-box-smart-style]');
  if (style) style.href = styleURL;
  else document.head.appendChild(E('link',{rel:'stylesheet',href:styleURL,'data-sing-box-smart-style':''}));
  const status = E('span', {class:'sbs-status-text'}, ['读取状态…']);
  const dot = E('i', {class:'sbs-status-dot'});
  const statusLine = E('div',{class:'sbs-status'},[dot,status]);
  const auto = E('input',{type:'checkbox',checked:data[1].code === 0});
  const autoLabel = E('label',{class:'sbs-autostart'},[auto,E('span',{},['开机自动启动'])]);
  let savedConfig={};
  try {if(data[2].code!==0)throw Error(data[2].stderr||'读取核心配置失败');savedConfig=JSON.parse(data[2].stdout||'{}');}
  catch(error){ui.addNotification(null,E('p',{},[error.message]));}
  const monitorLink = E('a',{class:'sbs-button sbs-open',href:window.location.protocol+'//'+window.location.hostname+':9797/',target:'_blank',rel:'noopener'},['监控面板']);
  const versionInfo = {};
  (data[3]?.stdout||'').split('\n').forEach(function(line){const pair=line.split('=');if(pair.length>1)versionInfo[pair[0]]=pair.slice(1).join('=').trim();});
  const currentVersion = versionInfo.VERSION || versionInfo.CORE || versionInfo.LUCI || '未知';
  function setMonitorAddress(settings){const port=(settings.listen||'').match(/:(\d+)$/);if(port)monitorLink.href=window.location.protocol+'//'+window.location.hostname+':'+port[1]+'/';}
  setMonitorAddress(savedConfig.service||{});
  const themeChoice = localStorage.getItem('sing-box-smart-luci-theme') || 'system';
  const themeButtons = [];
  let themeBlock;
  function applyTheme(value) {
   const selected = ['light','dark','system'].includes(value) ? value : 'system';
   const page = document.querySelector('.sbs-page');
   if(page) { if(selected === 'system') page.removeAttribute('data-theme'); else page.dataset.theme = selected; }
   localStorage.setItem('sing-box-smart-luci-theme',selected);
   themeButtons.forEach(function(button){const active=button.dataset.themeChoice===selected;button.classList.toggle('active',active);button.setAttribute('aria-pressed',String(active));});
  }
  ['浅色','深色','系统'].forEach(function(label,index){
   const value=['light','dark','system'][index];
   themeButtons.push(E('button',{type:'button',class:'cbi-button sbs-button sbs-theme-choice','data-theme-choice':value,'aria-pressed':'false',click:function(){applyTheme(value);}},[label]));
  });
  themeBlock = E('section',{class:'sbs-control-block sbs-theme-block','aria-labelledby':'sbs-theme-title'},[
   E('div',{class:'sbs-card-title'},[E('span',{id:'sbs-theme-title'},['主题'])]),
   E('div',{class:'sbs-theme-actions'},themeButtons)
  ]);
  const actions = [];
  const updateActions = [];
  const update = function(result) {
   const service = result && result[serviceName];
   const instances = service && service.instances && typeof service.instances === 'object' ? service.instances : {};
   const running = Object.keys(instances).some(k => instances[k].running);
   status.textContent = running ? '运行中' : '已停止';
   dot.classList.toggle('is-running',running);
   dot.classList.toggle('is-stopped',!running);
   return running;
  };
  let busy = false;
  const setBusy = function(value) {
   busy = value;
   actions.concat(updateActions).forEach(b => b.disabled = value);
   auto.disabled = value;
  };
  const run = function(action) {
   if (busy) return Promise.resolve();
   setBusy(true);
   return fs.exec(initPath,[action]).then(function(r) {
    if (r.code !== 0) throw new Error(r.stderr || '操作失败');
    return list(serviceName).then(update);
   }).catch(function(e) { ui.addNotification(null,E('p',{},[e.message])); })
   .finally(function() { setBusy(false); });
  };
  ['启动','停止','重启'].forEach(function(label,i) {
   actions.push(E('button',{type:'button',class:'cbi-button sbs-button sbs-action '+(['start','stop','restart'][i]),click:function(){return run(['start','stop','restart'][i]);}},[label]));
  });
  let updateState='idle';
  let availableVersion='';
  const versionText = E('span',{class:'sbs-version-text'},['当前版本 · '+currentVersion]);
  const updateOutput = E('pre',{class:'sbs-update-output',hidden:true,'aria-live':'polite'});
  const outputText = function(result){return [result?.stdout,result?.stderr].filter(Boolean).join('\n').trim();};
  const showOutput = function(result){
   const text=outputText(result).split('\n').filter(function(line){return !/^(VERSION|CORE|LUCI|UPDATES)=/.test(line);}).join('\n').trim();
   updateOutput.textContent=text;
   updateOutput.hidden=!text;
  };
  const updateButton = E('button',{type:'button',class:'cbi-button sbs-button',click:function(){
   if(busy)return;
   if(updateState==='available'){
    if(!confirm('确定升级 sing-box-smart 和 LuCI 软件包吗？'))return;
    updateState='updating';setBusy(true);updateButton.classList.add('start');versionText.textContent='正在更新…';updateButton.textContent='更新中…';
    fs.exec(updatePath,['update']).then(function(r){
     showOutput(r);
     if(r.code!==0)throw new Error(r.stderr||'更新失败');
     versionText.textContent='更新完成，页面即将刷新…';
     setTimeout(function(){window.location.reload();},2200);
    }).catch(function(e){updateState='error';updateButton.classList.remove('start');updateButton.textContent='重新检查';versionText.textContent='更新失败';ui.addNotification(null,E('p',{},[e.message]));setBusy(false);});
    return;
   }
   updateState='checking';setBusy(true);versionText.textContent='正在检查更新…';updateButton.textContent='检查中…';updateOutput.hidden=true;updateOutput.textContent='';
   fs.exec(updatePath,['check']).then(function(r){
    showOutput(r);
    if(r.code!==0)throw new Error(r.stderr||'检查更新失败');
    const match=(r.stdout||'').split('\n').find(line=>line.indexOf('UPDATES=')===0);
    const updates=match?match.slice(8).trim():'';
    if(updates&&updates!=='none'){
     availableVersion=updates.split(/[\s,]+/)[0].replace(/^sing-box-smart-/,'').replace(/^luci-app-sing-box-smart-/,'');
     updateState='available';updateButton.classList.add('start');versionText.textContent='当前版本 · '+currentVersion+' · 可更新至 '+availableVersion;updateButton.textContent='立即更新';
    } else {updateState='current';updateButton.classList.remove('start');versionText.textContent='当前版本 · '+currentVersion+' · 已是最新';updateButton.textContent='重新检查';}
   }).catch(function(e){updateState='error';updateButton.classList.remove('start');updateButton.textContent='重新检查';versionText.textContent='检查失败';ui.addNotification(null,E('p',{},[e.message]));}).finally(function(){setBusy(false);});
  }},['检查更新']);
  updateActions.push(updateButton);
  const versionBlock = E('section',{class:'sbs-control-block sbs-version-block','aria-labelledby':'sbs-version-title'},[
   E('div',{class:'sbs-card-title'},[E('span',{id:'sbs-version-title'},['版本与更新'])]),
   E('div',{class:'sbs-version-row'},[versionText,E('div',{class:'sbs-version-actions'},[updateButton])]),
   updateOutput
  ]);
  auto.addEventListener('change', function() {
   if (busy) return;
   setBusy(true);
   fs.exec(initPath,[auto.checked ? 'enable' : 'disable']).then(function(r) {
    if(r.code!==0)throw new Error(r.stderr || '保存失败');
   }).catch(function(e){auto.checked=!auto.checked;ui.addNotification(null,E('p',{},[e.message]));}).finally(function(){setBusy(false);});
  });
  update(data[0]);
  setTimeout(function(){applyTheme(themeChoice);},0);
  poll.add(function(){return list(serviceName).then(update);},3);
  return E('div',{class:'sbs-page'},[
   E('div',{class:'sbs-heading'},[E('h2',{},['sing-box-smart']),statusLine]),
   E('div',{class:'sbs-grid'},[
    E('section',{class:'sbs-card sbs-main-card'},[
     E('section',{class:'sbs-control-block','aria-labelledby':'sbs-control-title'},[
      E('div',{class:'sbs-card-title'},[E('span',{id:'sbs-control-title'},['服务控制'])]),
      E('div',{class:'sbs-actions'},[...actions,monitorLink]),
      E('div',{class:'sbs-auto-row'},[autoLabel])
     ]),
     themeBlock,
     versionBlock,
     coreConfig.render(savedConfig,function(settings){
      if(busy)throw Error('服务正忙，请稍后重试');
      setMonitorAddress(settings.service||{});
      setBusy(true);
      return fs.exec(initPath,['restart']).then(function(result){if(result.code!==0)throw Error(result.stderr||'重启失败，设置已保存');return list(serviceName).then(update);}).finally(function(){setBusy(false);});
     },function(){return !busy;})
    ]),
   ])
  ]);
 },
 handleSave:null, handleSaveApply:null, handleReset:null
});
