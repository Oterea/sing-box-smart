'use strict';
'require view';
'require rpc';
'require fs';
'require poll';
'require ui';
const serviceName = 'sing-box-smart';
const initPath = '/etc/init.d/' + serviceName;
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
  const autoLabel = E('label',{class:'sbs-switch'},[auto,E('span',{class:'sbs-switch-track'}),E('span',{},['开机自动启动'])]);
  const actions = [];
  const update = function(result) {
   const instances = (result[serviceName] || {}).instances || {};
   const running = Object.keys(instances).some(k => instances[k].running);
   status.textContent = running ? '运行中' : '已停止';
   dot.classList.toggle('is-running',running);
   dot.classList.toggle('is-stopped',!running);
   return running;
  };
  const run = function(action) {
   actions.forEach(b => b.disabled = true);
   return fs.exec(initPath,[action]).then(function(r) {
    if (r.code !== 0) throw new Error(r.stderr || '操作失败');
    return list(serviceName).then(update);
   }).catch(function(e) { ui.addNotification(null,E('p',{},[e.message])); })
   .finally(function() { actions.forEach(b => b.disabled = false); });
  };
  ['启动','停止','重启'].forEach(function(label,i) {
   actions.push(E('button',{class:'cbi-button sbs-action '+(['start','stop','restart'][i]),click:function(){return run(['start','stop','restart'][i]);}},[label]));
  });
  auto.addEventListener('change', function() {
   auto.disabled = true;
   fs.exec(initPath,[auto.checked ? 'enable' : 'disable']).then(function(r) {
    if(r.code!==0)throw new Error(r.stderr || '保存失败');
   }).catch(function(e){auto.checked=!auto.checked;ui.addNotification(null,E('p',{},[e.message]));}).finally(function(){auto.disabled=false;});
  });
  update(data[0]);
  poll.add(function(){return list(serviceName).then(update);},3);
  return E('div',{class:'sbs-page'},[
   E('div',{class:'sbs-heading'},[E('div',{},[E('div',{class:'sbs-kicker'},['SERVICE CONTROL']),E('h2',{},['sing-box-smart']),E('p',{},['节点监测与自动选择'])]),statusLine]),
   E('div',{class:'sbs-grid'},[
    E('section',{class:'sbs-card sbs-main-card'},[
     E('div',{class:'sbs-card-title'},[E('span',{},['服务控制']),E('span',{class:'sbs-live-label'},['OpenWrt procd'])]),
     E('div',{class:'sbs-actions'},actions),
     E('div',{class:'sbs-divider'}),
     autoLabel,
     E('p',{class:'sbs-hint'},['开机后自动启动 sing-box-smart，并由系统监控进程。'])
    ]),
    E('section',{class:'sbs-card sbs-link-card'},[
     E('div',{class:'sbs-card-title'},[E('span',{},['监控面板']),E('span',{class:'sbs-arrow'},['↗'])]),
     E('p',{},['查看节点评分、延迟历史和自动切换状态。']),
     E('a',{class:'sbs-open',href:window.location.protocol+'//'+window.location.hostname+':9797/',target:'_blank',rel:'noopener'},['打开 smart 面板',' ↗'])
    ])
   ]),
   E('div',{class:'sbs-note'},[E('b',{},['配置说明']),E('span',{},['sing-box API 地址和节点检测设置在 smart 面板中修改。'])])
  ]);
 },
 handleSave:null, handleSaveApply:null, handleReset:null
});
