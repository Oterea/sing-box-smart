'use strict';
'require view';
'require rpc';
'require fs';
'require poll';
'require ui';
const serviceName = 'sing-box-smart';
const initPath = '/etc/init.d/' + serviceName;
const updateCheckPath = '/usr/libexec/sing-box-smart-update-check';
const updateStartPath = '/usr/libexec/sing-box-smart-update-start';
const updateStatusPath = '/usr/libexec/sing-box-smart-update-status';
const list = rpc.declare({object:'service',method:'list',params:['name'],expect:{'':{}}});
return view.extend({
 load: function() { return Promise.all([list(serviceName), fs.exec(initPath,['enabled'])]); },
 render: function(data) {
  const style = E('link',{rel:'stylesheet',href:L.resource('sing-box-smart/style.css')});
  if (!document.querySelector('link[href*="sing-box-smart/style.css"]')) document.head.appendChild(style);
  const status = E('span', {class:'sbs-status-text'}, ['读取状态…']);
  const dot = E('i', {class:'sbs-status-dot'});
  const statusLine = E('div',{class:'sbs-status'},[dot,status]);
  const auto = E('input',{type:'checkbox',checked:data[1].code === 0});
  const autoTrack = E('button',{type:'button',class:'sbs-switch-track',ariaLabel:'开机自动启动'});
  const autoLabel = E('div',{class:'sbs-switch'},[auto,autoTrack,E('span',{class:'sbs-switch-text'},['开机自动启动'])]);
  const monitorLink = E('a',{class:'sbs-open',href:window.location.protocol+'//'+window.location.hostname+':9797/',target:'_blank',rel:'noopener'},['打开监控面板']);
  const actions = [];
  const updateText = E('span',{class:'sbs-update-text'},['正在检查更新…']);
  const updateButton = E('button',{type:'button',class:'cbi-button sbs-update-button',disabled:true},['检查更新']);
  const updateRow = E('div',{class:'sbs-update-row'},[E('span',{},['面板更新']),updateText,updateButton]);
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
  let updating = false;
  let reloadScheduled = false;
  const setBusy = function(value) {
   busy = value;
   actions.forEach(b => b.disabled = value);
   auto.disabled = value;
   autoTrack.disabled = value;
   updateButton.disabled = value || updateButton.dataset.available !== 'yes';
  };
  const readUpdate = function() {
   return fs.exec(updateCheckPath,[]).then(function(r) {
    if (r.code !== 0) throw new Error(r.stderr || '更新检查失败');
    const result = JSON.parse(r.stdout || '{}');
    updateText.textContent = result.available ? '发现 '+result.version+'（当前 '+(result.current||'未知')+'）' : '已是最新版本 · '+(result.current||result.version||'未知');
    updateText.classList.toggle('is-available',Boolean(result.available));
    updateButton.dataset.available = result.available ? 'yes' : 'no';
    updateButton.textContent = result.available ? '立即更新' : '检查更新';
    updateButton.disabled = !result.available;
   }).catch(function() {
    updateText.textContent = '暂时无法检查';
    updateButton.dataset.available = 'no';
    updateButton.disabled = true;
   });
  };
  const showUpdateStatus = function(result) {
   if (!result || !result.state) return;
   if (result.state === 'idle') return;
   const active = ['starting','downloading','verifying','installing','restarting'].includes(result.state);
   if (!active && !updating) return;
   if (active) { updating = true; setBusy(true); }
   const progress = Number(result.progress) || 0;
   updateText.textContent = (result.message || '正在更新…')+' · '+progress+'%';
   if (result.state === 'success') {
    if (reloadScheduled) return;
    reloadScheduled = true;
    updateText.textContent = '更新完成 · 100%，正在刷新…';
    setTimeout(function(){window.location.reload();},1200);
   } else if (result.state === 'error') {
    updating = false;
    updateText.textContent = result.message || '更新失败';
    setBusy(false);
   }
  };
  const pollUpdateStatus = function() {
   return fs.exec(updateStatusPath,[]).then(function(r) {
    if (r.code !== 0) throw new Error(r.stderr || '无法读取更新状态');
    const result = JSON.parse(r.stdout || '{}');
    showUpdateStatus(result);
    return result;
   });
  };
  updateButton.addEventListener('click',function() {
   if (busy || updateButton.dataset.available !== 'yes' || !window.confirm('更新 LuCI 面板和 smart 核心？现有配置会保留。')) return;
   setBusy(true);
   updating = true;
   fs.exec(updateStartPath,[]).then(function(r) {
    if (r.code !== 0) throw new Error(r.stderr || '无法启动更新');
    updateButton.disabled = true;
    updateText.textContent = '正在准备更新…';
   }).catch(function(e) {
    updating = false;
    ui.addNotification(null,E('p',{},[e.message]));
    setBusy(false);
   });
  });
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
   actions.push(E('button',{class:'cbi-button sbs-action '+(['start','stop','restart'][i]),click:function(){return run(['start','stop','restart'][i]);}},[label]));
  });
  auto.addEventListener('change', function() {
   if (busy) return;
   setBusy(true);
   fs.exec(initPath,[auto.checked ? 'enable' : 'disable']).then(function(r) {
    if(r.code!==0)throw new Error(r.stderr || '保存失败');
   }).catch(function(e){auto.checked=!auto.checked;ui.addNotification(null,E('p',{},[e.message]));}).finally(function(){setBusy(false);});
  });
  autoTrack.addEventListener('click', function() { if (!busy) { auto.checked = !auto.checked; auto.dispatchEvent(new Event('change')); } });
  update(data[0]);
  pollUpdateStatus().then(function() { if (!updating) return readUpdate(); }).catch(function() { return readUpdate(); });
  poll.add(pollUpdateStatus,1);
  poll.add(function(){return list(serviceName).then(update);},3);
  return E('div',{class:'sbs-page'},[
   E('div',{class:'sbs-heading'},[E('h2',{},['sing-box-smart'])]),
   E('div',{class:'sbs-grid'},[
    E('section',{class:'sbs-card sbs-main-card'},[
     E('div',{class:'sbs-card-title'},[E('span',{},['服务控制']),statusLine]),
     E('div',{class:'sbs-actions'},actions),
     E('div',{class:'sbs-divider'}),
     E('div',{class:'sbs-auto-row'},[autoLabel,monitorLink]),
     updateRow,
     E('p',{class:'sbs-hint'},['开机后自动启动并监控服务。'])
    ]),

   ]),
   E('div',{class:'sbs-note'},['API 地址、根策略组和匹配正则在 smart 面板中修改。'])
  ]);
 },
 handleSave:null, handleSaveApply:null, handleReset:null
});
