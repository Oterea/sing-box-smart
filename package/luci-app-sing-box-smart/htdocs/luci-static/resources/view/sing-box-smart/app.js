'use strict';
'require view';
'require rpc';
'require fs';
'require poll';
'require ui';
'require sing-box-smart.coreconfig as coreConfig';
const serviceName = 'sing-box-smart';
const initPath = '/etc/init.d/' + serviceName;
const list = rpc.declare({object:'service',method:'list',params:['name'],expect:{'':{}}});
return view.extend({
 load: function() { return Promise.all([list(serviceName), fs.exec(initPath,['enabled']), fs.exec('/usr/bin/sing-box-smart',['config-settings','read'])]); },
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
  function setMonitorAddress(settings){const port=(settings.listen||'').match(/:(\d+)$/);if(port)monitorLink.href=window.location.protocol+'//'+window.location.hostname+':'+port[1]+'/';}
  setMonitorAddress(savedConfig.service||{});
  const actions = [];
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
   actions.forEach(b => b.disabled = value);
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
  auto.addEventListener('change', function() {
   if (busy) return;
   setBusy(true);
   fs.exec(initPath,[auto.checked ? 'enable' : 'disable']).then(function(r) {
    if(r.code!==0)throw new Error(r.stderr || '保存失败');
   }).catch(function(e){auto.checked=!auto.checked;ui.addNotification(null,E('p',{},[e.message]));}).finally(function(){setBusy(false);});
  });
  update(data[0]);
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
