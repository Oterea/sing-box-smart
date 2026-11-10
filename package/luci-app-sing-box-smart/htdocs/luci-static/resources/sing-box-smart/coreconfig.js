'use strict';
'require fs';
'require ui';

function field(form, controls, key, label, help, value, type) {
 const input=E('input',{id:'sbs-core-'+key,'aria-describedby':'sbs-core-help-'+key,type:type||'text',value:value||'',autocomplete:'off',class:'cbi-input-text'});
 controls[key]=input;
 form.appendChild(E('div',{class:'sbs-service-field'},[
  E('label',{'for':'sbs-core-'+key},[label]),
  E('p',{id:'sbs-core-help-'+key,class:'sbs-setting-help'},[help]),input
 ]));
}

function secretControl(input) {
 const toggle=E('button',{type:'button',class:'cbi-button sbs-button sbs-secret-toggle','aria-controls':input.id,'aria-pressed':'false','aria-label':'显示 gRPC 密钥'},['显示']);
 toggle.addEventListener('click',function(){
  const visible=input.type==='password';
  input.type=visible?'text':'password';
  toggle.textContent=visible?'隐藏':'显示';
  toggle.setAttribute('aria-pressed',String(visible));
  toggle.setAttribute('aria-label',visible?'隐藏 gRPC 密钥':'显示 gRPC 密钥');
 });
 return E('div',{class:'sbs-secret-control'},[input,toggle]);
}

return L.Class.extend({
 render: function(initial,onApply,canSave) {
  const form=E('div',{class:'sbs-service-settings'});
  const controls={};
  const connection=initial.connection||{};
  const service=initial.service||{};
  field(form,controls,'api','sing-box API 地址','smart 启动时必须能访问该地址。',connection.api);
  field(form,controls,'root','根策略组','必须存在于 sing-box Clash API。',connection.root);
  field(form,controls,'pattern','策略组匹配规则','用于发现可监控的 selector 策略组。',connection.pattern);
  field(form,controls,'listen','smart 监听地址','端口冲突会导致 smart 核心无法启动。',service.listen);
  field(form,controls,'grpc_address','gRPC 地址','仅用于睡眠监控；不使用时可以留空。',service.grpc_address);
  const secret=E('input',{id:'sbs-core-grpc_secret','aria-describedby':'sbs-core-help-grpc_secret',type:'password',value:service.grpc_secret||'',autocomplete:'off',class:'cbi-input-text'});
  controls.grpc_secret=secret;
  form.appendChild(E('div',{class:'sbs-service-field'},[
   E('label',{'for':'sbs-core-grpc_secret'},['gRPC 密钥']),
   E('p',{id:'sbs-core-help-grpc_secret',class:'sbs-setting-help'},['与 sing-box 原生 gRPC API 保持一致。']),
   secretControl(secret)
  ]));
  field(form,controls,'log_dir','日志目录','目录必须是绝对路径且可写。',service.log_dir);
  const status=E('p',{class:'sbs-setting-help'},['运行参数由 9797 运行设置管理；这里仅修改核心启动和连接配置。']);
  const check=E('button',{type:'button',class:'cbi-button sbs-button'},['检查 sing-box']);
  check.addEventListener('click',function(){
   if(!canSave()){ui.addNotification(null,E('p',{},['服务正忙，请稍后检查。']));return;}
   check.disabled=true;
   fs.exec('/usr/bin/sing-box-smart',['config-settings','check']).then(function(result){
    if(result.code!==0)throw Error(result.stderr||'sing-box 配置检查失败');
    let checked={}; try { checked=JSON.parse(result.stdout||'{}'); } catch(e) {}
    ui.addNotification(null,E('p',{},['sing-box 配置检查通过，发现 '+(checked.selectors||0)+' 个策略组。']));
   }).catch(function(error){ui.addNotification(null,E('p',{},[error.message]));})
    .finally(function(){check.disabled=false;});
  });
  const save=E('button',{type:'button',class:'cbi-button sbs-button sbs-settings-save'},['保存并重启']);
  save.addEventListener('click',function(){
   if(!canSave()){ui.addNotification(null,E('p',{},['服务正忙，请稍后保存。']));return;}
   const next=JSON.parse(JSON.stringify(initial));
   next.connection=next.connection||{};
   next.service=next.service||{};
   next.connection.api=controls.api.value.trim();
   next.connection.root=controls.root.value.trim();
   next.connection.pattern=controls.pattern.value.trim();
   next.service.listen=controls.listen.value.trim();
   next.service.grpc_address=controls.grpc_address.value.trim();
   next.service.grpc_secret=controls.grpc_secret.value;
   next.service.log_dir=controls.log_dir.value.trim();
   save.disabled=true;
   fs.write('/etc/sing-box-smart/config.pending.json',JSON.stringify(next),384)
    .then(function(){return fs.exec('/usr/bin/sing-box-smart',['config-settings','apply']);})
    .then(function(result){if(result.code!==0)throw Error(result.stderr||'配置校验失败');return onApply(next);})
    .then(function(){ui.addNotification(null,E('p',{},['核心配置已保存并重启。']));})
    .catch(function(error){ui.addNotification(null,E('p',{},[error.message]));})
    .finally(function(){save.disabled=false;});
  });
  return E('section',{class:'sbs-service-details','aria-labelledby':'sbs-core-title'},[
   E('h3',{id:'sbs-core-title',class:'sbs-service-title'},['核心配置']),form,status,E('div',{class:'sbs-service-footer'},[check,save])
  ]);
 }
});
