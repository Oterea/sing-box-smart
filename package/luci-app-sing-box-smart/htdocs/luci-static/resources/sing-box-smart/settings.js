'use strict';
'require fs';
'require ui';
return L.Class.extend({
 render: function(initial,onApply,canSave,footerLeft) {
  const form=E('div',{class:'sbs-service-settings'});
  const controls={};
  const fields=[
   ['listen','监听地址','端口变更后，使用新地址打开监控面板。'],
   ['grpc_address','gRPC 地址','睡眠监控使用的 sing-box 原生 API 地址。'],
   ['grpc_secret','gRPC 密钥','与 sing-box 原生 API 一致，无密钥则留空。'],
   ['log_dir','日志目录','自动轮转保留当前日志和一个旧文件。']
  ];
  fields.forEach(function(field){
   const input=E('input',{id:'sbs-service-'+field[0],'aria-describedby':'sbs-help-'+field[0],type:field[0]==='grpc_secret'?'password':'text',value:initial[field[0]]||'',autocomplete:'off',class:'cbi-input-text'});controls[field[0]]=input;
   form.appendChild(E('div',{class:'sbs-service-field'},[
    E('label',{'for':'sbs-service-'+field[0]},[field[1]]),
    E('p',{id:'sbs-help-'+field[0],class:'sbs-setting-help'},[field[2]]),input
   ]));
  });
  const save=E('button',{type:'button',class:'cbi-button sbs-button sbs-settings-save'},['保存并重启']);
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
  const footer=E('div',{class:'sbs-service-footer'},footerLeft?[footerLeft,save]:[save]);
  return E('section',{class:'sbs-service-details','aria-labelledby':'sbs-service-title'},[E('h3',{id:'sbs-service-title',class:'sbs-service-title'},['服务设置']),form,footer]);
 }
});
