'use strict';
'require fs';
'require ui';
return L.Class.extend({
 render: function(initial,onApply,canSave) {
  const form=E('div',{class:'sbs-service-settings'});
  const controls={};
  const fields=[
   ['listen','监听地址','例如 0.0.0.0:9797；端口变更后请用新地址打开监控面板。'],
   ['grpc_address','gRPC 地址','sing-box 原生 API 地址，供睡眠监控使用，例如 127.0.0.1:9090。'],
   ['grpc_secret','gRPC 密钥','与 sing-box 原生 API 的密钥一致；未设置密钥时留空。'],
   ['log_dir','日志目录','运行日志保存目录；自动轮转保留当前文件和一个旧文件。']
  ];
  fields.forEach(function(field){
   const input=E('input',{type:field[0]==='grpc_secret'?'password':'text',value:initial[field[0]]||'',autocomplete:'off',class:'cbi-input-text'});controls[field[0]]=input;
   form.appendChild(E('div',{class:'sbs-service-field'},[
    E('label',{},[field[1],input]),E('details',{class:'sbs-setting-help'},[E('summary',{},['说明']),E('p',{},[field[2]])])
   ]));
  });
  const save=E('button',{type:'button',class:'cbi-button sbs-update-button'},['保存并重启']);
  save.addEventListener('click',function(){
   if(!canSave()){ui.addNotification(null,E('p',{},['服务正忙，请稍后保存。']));return;}
   const settings={};Object.keys(controls).forEach(k=>settings[k]=controls[k].value.trim());
   // Preserve leading/trailing spaces in a secret.
   settings.grpc_secret=controls.grpc_secret.value;
   save.disabled=true;
   fs.write('/etc/sing-box-smart/service.pending.json',JSON.stringify(settings),384).then(function(){
    return fs.exec('/usr/bin/sing-box-smart',['service-settings','apply']);
   }).then(function(result){
    if(result.code!==0)throw Error(result.stderr||'保存失败');
    return onApply(settings);
   }).then(function(){ui.addNotification(null,E('p',{},['服务设置已保存并重启。']));})
   .catch(function(error){ui.addNotification(null,E('p',{},[error.message]));})
   .finally(function(){save.disabled=false;});
  });
  form.appendChild(save);
  return E('details',{class:'sbs-service-details'},[E('summary',{},['服务设置']),form]);
 }
});
