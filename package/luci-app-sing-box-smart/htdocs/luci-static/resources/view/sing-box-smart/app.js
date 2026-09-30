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
  const status = E('span', {}, ['读取状态…']);
  const auto = E('input',{type:'checkbox',checked:data[1].code === 0});
  const actions = [];
  const update = function(result) {
   const instances = (result[serviceName] || {}).instances || {};
   status.textContent = Object.keys(instances).some(k => instances[k].running) ? '运行中' : '已停止';
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
   actions.push(E('button',{class:'cbi-button cbi-button-action',click:function(){return run(['start','stop','restart'][i]);}},[label]));
  });
  auto.addEventListener('change', function() {
   auto.disabled = true;
   fs.exec(initPath,[auto.checked ? 'enable' : 'disable']).then(function(r) {
    if(r.code!==0)throw new Error(r.stderr || '保存失败');
   }).catch(function(e){auto.checked=!auto.checked;ui.addNotification(null,E('p',{},[e.message]));}).finally(function(){auto.disabled=false;});
  });
  update(data[0]);
  poll.add(function(){return list(serviceName).then(update);},3);
  return E('div',{},[
   E('h2',{},['sing-box-smart']),
   E('div',{class:'cbi-section'},[
    E('p',{},['服务状态：',status]),
    E('div',{class:'cbi-section-actions'},actions),
    E('p',{},[E('label',{},[auto,' 开机自动启动'])]),
    E('p',{},[E('a',{class:'cbi-button cbi-button-action',href:window.location.protocol+'//'+window.location.hostname+':9797/',target:'_blank',rel:'noopener'},['打开 smart 面板'])]),
    E('p',{},['sing-box API 地址在 smart 面板的设置中修改。'])
   ])
  ]);
 },
 handleSave:null, handleSaveApply:null, handleReset:null
});
