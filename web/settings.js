// Runtime settings UI. The app renderer supplies snapshots; this module owns
// form editing only and never overwrites unsaved edits with SSE updates.
const SmartSettings = (() => {
 let current, dirty=false, saving=false;
 const groups=[
 ['检测',[
 ['test_url','探测地址','url','用于测量节点连通性和延迟，建议选择小体积响应地址。'],
 ['current_seconds','当前节点间隔（秒）','number','当前正在使用的节点多久检查一次。',1,86400],
 ['candidate_seconds','候选节点间隔（秒）','number','分数较好的备用节点多久检查一次。',1,86400],
 ['ordinary_seconds','普通节点间隔（秒）','number','其他节点多久检查一次。',1,86400],
 ['timeout_seconds','探测超时（秒）','number','超过此时间未完成的探测记为超时。',1,120]
 ]],
 ['睡眠',[
 ['sleep_enabled','自动睡眠','checkbox','低流量时暂停探测，恢复使用网络后自动唤醒。需配置 gRPC。'],
 ['sleep_idle_seconds','睡眠观察时长（分钟）','number','整个观察窗口内的有效流量低于允许值时进入睡眠。',1,1440,60],
 ['quiet_bytes','睡眠允许流量（KiB）','number','观察窗口内允许的总流量；探测和 smart 面板流量不计入。',1/1024,1048576,1024],
 ['wake_window_seconds','唤醒窗口（秒）','number','统计单个连接最近这段时间的传输量。',1,60],
 ['wake_bytes','唤醒流量（MiB）','number','单个连接在唤醒窗口达到此流量时恢复探测，不合并不同连接。',1/1048576,1024,1048576]
 ]],
 ['日志',[
 ['log_level','日志级别','radio','INFO 记录日常运行；DEBUG 额外记录探测细节。日志自动轮转，不会无限保留。']
 ]],
 ['显示',[
 ['node_display_limit','默认显示节点数','number','默认展示分数排名靠前的节点；其余可点击显示全部，所有节点仍继续检测和参与选择。',1,1000],
 ['long_term_display_limit','每组长期表现显示数','number','长期表现中每个稳定性分组默认显示的节点数；其余可点击显示全部。',1,1000],
 ['delay_green_ms','绿色延迟上限（ms）','number','小于或等于此值显示绿色。仅影响显示，不改变评分。',1,59999],
 ['delay_yellow_ms','黄色延迟上限（ms）','number','超过绿色上限且不超过此值显示黄色，更高显示红色。',2,60000]
 ]],
 ['高级',[
 ['startup_seconds','启动观察（秒）','number','启动或切换机场后连续检查，观察结束再自动选择。',1,300],
 ['resume_seconds','恢复观察（秒）','number','继续检测或睡眠唤醒后进行短暂连续检查。',1,300],
 ['retry_seconds','故障重试间隔（秒）','number','全部节点不可用或管理接口异常时的重试间隔。',1,3600],
 ['switch_ratio','切换分数比例','number','当前分数达到目标节点分数的此倍数时考虑切换；分数越低越好。',1.01,10],
 ['recovery_drop_ratio','恢复延迟比例','number','新延迟下降至原延迟的此比例以下，才考虑恢复复查。',0.01,0.99],
 ['recovery_drop_min_ms','恢复最少下降（ms）','number','延迟还需至少下降此值，避免小幅变化触发复查。',1,60000],
 ['recovery_good_max_ms','恢复良好延迟上限（ms）','number','失败后成功的节点需达到此延迟要求，才触发恢复复查。',1,60000],
 ['recovery_steps_seconds','恢复复查间隔（秒）','text','依次执行的复查间隔，用逗号分隔，例如 3,6,12,20。'],
 ['history_limit','历史样本数量','number','每个节点在内存中保留的样本数；图表最多显示最近 20 条，减少会裁剪旧样本。',1,200]
 ]]
 ];
 const fields=groups.flatMap(g=>g[1]);
 const byID=id=>document.getElementById(id);
 function mount(){
 const form=byID('runtime-form'),tabs=byID('settings-tabs');if(!form||!tabs)return;
 const panels=document.createElement('div');panels.className='settings-scroll';form.append(panels);
 function activate(tab,focus=false){
 for(const button of tabs.querySelectorAll('[role=tab]')){
 const selected=button===tab;button.setAttribute('aria-selected',String(selected));button.tabIndex=selected?0:-1;
 byID(button.getAttribute('aria-controls')).hidden=!selected;
 }
 form.hidden=tab.id==='settings-tab-connection';
 if(focus)tab.focus();
 }
 for(const [index,[title,items]] of groups.entries()){
 const section=document.createElement('section');section.className='settings-group';section.id='settings-panel-'+index;section.hidden=true;
 section.setAttribute('role','tabpanel');section.setAttribute('aria-labelledby','settings-tab-'+index);
 const tab=document.createElement('button');tab.id='settings-tab-'+index;tab.type='button';tab.className='settings-tab';tab.textContent=title;tab.tabIndex=-1;
 tab.setAttribute('role','tab');tab.setAttribute('aria-selected','false');tab.setAttribute('aria-controls',section.id);tabs.append(tab);
 const grid=document.createElement('div');grid.className='runtime-grid';
 for(const [key,label,type,help,min,max,scale] of items){
 const field=document.createElement(type==='radio'?'fieldset':'div');field.className='runtime-field';
 const name=document.createElement(type==='radio'?'legend':'label');if(type!=='radio')name.htmlFor='setting-'+key;name.textContent=label;field.append(name);
 const input=document.createElement(type==='radio'?'div':'input');input.id='setting-'+key;
 if(type==='radio'){
 input.className='setting-choices';
 for(const level of ['info','debug','warn','error']){const option=document.createElement('input');option.type='radio';option.name=key;option.value=level;option.required=true;const choice=document.createElement('label');choice.className='setting-choice';const text=document.createElement('span');text.textContent=level.toUpperCase();choice.append(option,text);input.append(choice);}
 }
 else {input.type=type;input.name=key;if(type!=='checkbox')input.required=true;}
 if(type==='number'){input.min=min;input.max=max;input.step=(scale||key.includes('ratio'))?'any':'1';}
 input.addEventListener('input',()=>{dirty=true;});
 const text=document.createElement('p');text.id='setting-help-'+key;text.className='setting-help';text.textContent=help;
 input.setAttribute('aria-describedby',text.id);field.append(text,input);grid.append(field);
 }section.append(grid);panels.append(section);
 }
 tabs.addEventListener('click',event=>{const tab=event.target.closest('[role=tab]');if(tab&&tabs.contains(tab))activate(tab);});
 tabs.addEventListener('keydown',event=>{
 const buttons=[...tabs.querySelectorAll('[role=tab]')],index=buttons.indexOf(event.target);if(index<0)return;
 let next;if(event.key==='ArrowRight')next=(index+1)%buttons.length;else if(event.key==='ArrowLeft')next=(index+buttons.length-1)%buttons.length;else if(event.key==='Home')next=0;else if(event.key==='End')next=buttons.length-1;else return;
 event.preventDefault();activate(buttons[next],true);
 });
 // Invalid fields in another category must be revealed before browser focus.
 form.addEventListener('invalid',event=>{const panel=event.target.closest('[role=tabpanel]');if(panel)activate(byID(panel.getAttribute('aria-labelledby')));},true);
 const actions=document.createElement('div');actions.className='settings-actions settings-dialog-actions';
 const reset=document.createElement('button');reset.type='button';reset.className='button secondary';reset.textContent='撤销修改';
 reset.addEventListener('click',()=>{dirty=false;fill();});
 const submit=document.createElement('button');submit.type='submit';submit.className='button secondary';submit.textContent='保存并应用';actions.append(reset,submit);form.append(actions);
 form.addEventListener('submit',async event=>{
 event.preventDefault();if(saving||!current)return;
 const body={...current};
 for(const [key,,type,,,,scale] of fields){const input=byID('setting-'+key);body[key]=type==='radio'?input.querySelector('input:checked')?.value:type==='checkbox'?input.checked:type==='number'?(scale?Math.round(Number(input.value)*scale):Number(input.value)):key==='recovery_steps_seconds'?input.value.split(/[,，\s]+/).filter(Boolean).map(Number):input.value;}
 saving=true;form.querySelectorAll('input,select,button').forEach(el=>el.disabled=true);
 try {const response=await fetch('/api/settings',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify(body)});const result=await response.json();if(!response.ok)throw Error(result.error||'保存失败');current=body;dirty=false;fill();toast('运行设置已生效');}
 catch(error){toast(error.message);}
 finally {saving=false;form.querySelectorAll('input,select,button').forEach(el=>el.disabled=false);}
 });
 }
 function fill(){if(!current)return;for(const [key,,type,,,,scale] of fields){const input=byID('setting-'+key);if(!input)continue;if(type==='radio'){for(const option of input.querySelectorAll('input'))option.checked=option.value===current[key];}else if(type==='checkbox')input.checked=current[key];else input.value=key==='recovery_steps_seconds'?current[key].join(','):type==='number'?current[key]/(scale||1):current[key];}}
 function update(settings){if(!settings?.delay_green_ms)return;const changed=JSON.stringify(current)!==JSON.stringify(settings);current=settings;if(changed&&!dirty&&!saving)fill();}
 mount();return {update};
})();
