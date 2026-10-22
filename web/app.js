const $ = id => document.getElementById(id);
let nodeList;
let state, busy = false, toastTimer, airportKey = '', eventsKey = '', sleepRecordsKey = '', stateEtag = '', streamOpen = false, stream, lastStreamEvent = 0, stateRevision = -1, stateInstance = '', detailNodeId = '', detailData = null, detailTimer = 0, detailStream = null, detailRequestController = null, detailLongTermController = null, detailRequestToken = 0, detailUpdateChain = Promise.resolve(), detailNodeSequence = 0, detailSummaryKey = '', detailSamplesKey = '', detailView = 'realtime', detailFollowLatest = true;
const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const num = value => value == null ? '—' : value >= 10000 ? (value / 1000).toFixed(1) + 'k' : Math.round(value).toString();
// Large scores are useful internally for ranking, but are not meaningful in
// the compact table. Keep the exact value in state and show a quiet marker.
const scoreNum = value => value == null || !Number.isFinite(value) || value > 10000 ? '—' : Math.round(value).toString();
function animateNumber(el, value, formatter=num) {
 const next=Number.isFinite(value)?Math.round(value):null;
 const prev=Number(el.dataset.value);
 if(next==null){el.textContent=formatter(value);delete el.dataset.value;return;}
 if(!Number.isFinite(prev)||document.hidden||window.matchMedia('(prefers-reduced-motion: reduce)').matches){el.textContent=formatter(value);el.dataset.value=next;return;}
 const start=performance.now(), from=prev, duration=260;
 cancelAnimationFrame(Number(el.dataset.raf)||0);
 const step=now=>{const t=Math.min(1,(now-start)/duration), eased=1-Math.pow(1-t,3);el.textContent=formatter(from+(next-from)*eased);el.dataset.value=next;if(t<1)el.dataset.raf=requestAnimationFrame(step);};
 el.dataset.raf=requestAnimationFrame(step);
}
const time = value => new Date(value).toLocaleTimeString('zh-CN', {hour12:false});
const recordTime = value => {
 const date=new Date(value);if(!Number.isFinite(date.getTime()))return '—';
 const pad=value=>String(value).padStart(2,'0');
 return `${date.getFullYear()}-${pad(date.getMonth()+1)}-${pad(date.getDate())} ${time(value)}`;
};
const elapsedTime = value => {
 const total=Number.isFinite(Number(value))?Math.max(0,Math.floor(Number(value))):0;
 const hours=Math.floor(total/3600),minutes=Math.floor(total%3600/60),seconds=total%60;
 return [hours?`${hours} 小时`:'',minutes?`${minutes} 分钟`:'',seconds||!total?`${seconds} 秒`:''].filter(Boolean).join(' ');
};
const tone = (value,best) => value == null ? 'none' : value <= best*1.4 ? 'good' : value <= best*2.5 ? 'medium' : 'bad';
const delayTone = value => value == null ? 'none' : value <= (state?.settings?.delay_green_ms??200) ? 'good' : value <= (state?.settings?.delay_yellow_ms??600) ? 'medium' : 'bad';
const sampleKey = sample => JSON.stringify([sample.at,sample.success,sample.score,sample.delay_ms,sample.error]);
// Both sections share the same header, column definitions and row renderer.
function tableHeading() {
 return '<colgroup><col class="col-node"><col class="col-action"><col class="col-score"><col class="col-delay"><col class="col-history"><col class="col-history"><col class="col-detail"></colgroup><thead><tr><th scope="col">节点</th><th scope="col">检查次数</th><th scope="col">分数</th><th scope="col">延迟</th><th scope="col">分数历史</th><th scope="col">延迟历史</th><th scope="col">详情</th></tr></thead>';
}
function nodeTier(n,s) {
 return n.id&&n.id===s.current_id?'current':n.recovery_remaining>0?'recovery':n.tier==='candidate'?'candidate':'ordinary';
}
function nodeKey(n,best,s,selected=false) {
 // A changing best score matters only when it changes a visible color.
 // Including the raw best value would redraw every node's SVG on each update.
 const colors=(n.history??[]).slice(-20).map(sample=>tone(sample.score,best)).join(',');
 return JSON.stringify({id:n.id,name:n.name,tier:nodeTier(n,s),checks:n.checks,score:n.score,delay:n.delay_ms,overflow:n.score_overflow,last:n.last_success,current:selected||n.id===s.current_id,history:n.history,delayColors:[s.settings?.delay_green_ms,s.settings?.delay_yellow_ms],colors,rowColor:tone(n.score,best),busy,pending:s.pending_id,phase:s.phase,paused:s.paused,sleeping:s.sleeping,api:s.api_healthy,selection_mode:s.selection_mode,control_active:s.control_active});
}
function historyKey(n) {
 return JSON.stringify((n.history??[]).slice(-20).map(sample=>[sample.at,sample.success,sample.score,sample.delay_ms,sample.error]));
}
function nodeRow(n,best,s) {
 const current=Boolean(n.id)&&n.id===s.current_id;
 const disabled=busy||Boolean(s.pending_id)||s.paused||s.sleeping||s.phase==='startup'||!s.api_healthy||current||s.control_active===false||!n.id;
 const delayColor=n.checks&&!n.last_success?'bad':delayTone(n.delay_ms);
 const buttonText=n.id?n.name:'等待首次选择';
 return `<tr data-row-id="${esc(n.id||'selected')}" class="${current?'current-row':''}"><td><button class="node-choice tier-${nodeTier(n,s)}${current?' selected':''}" data-node="${esc(n.id)}" ${disabled?'disabled':''} title="${esc(n.name)}">${esc(buttonText)}</button></td><td><span class="check-count">${n.checks}</span></td><td><span class="metric-cell tone-${tone(n.score,best)}">${scoreNum(n.score)}</span></td><td><span class="metric-cell tone-${delayColor}">${n.delay_ms!=null?num(n.delay_ms):n.checks?'失败':'—'}</span></td><td>${history(n.history,'score',best)}</td><td>${history(n.history,'delay',best)}</td><td><button class="detail-button" data-detail-node="${esc(n.id)}" ${!n.id?'disabled':''}>详情</button></td></tr>`;
}
function detailPercent(value) {
 return Number.isFinite(Number(value)) ? (Number(value)*100).toFixed(1)+'%' : '—';
}
function detailMs(value) {
 return Number.isFinite(Number(value)) ? Math.round(Number(value))+' ms' : '—';
}
function patchRow(row,n,best,s,selected=false) {
 const current=Boolean(n.id)&&n.id===s.current_id;
 row.className=current?'current-row':'';
 const button=row.querySelector('button.node-choice');
 const disabled=busy||Boolean(s.pending_id)||s.paused||s.sleeping||s.phase==='startup'||!s.api_healthy||current||s.control_active===false||!n.id;
 button.textContent=n.id?n.name:'等待首次选择';
 button.title=n.name;
 button.disabled=disabled;
 button.className=`node-choice tier-${nodeTier(n,s)}${current?' selected':''}`;
 if(n.id)button.dataset.node=n.id;else delete button.dataset.node;
 animateNumber(row.cells[1].firstElementChild,n.checks);
 animateNumber(row.cells[2].firstElementChild,n.score_overflow?null:n.score,scoreNum);
 row.cells[2].firstElementChild.className=`metric-cell tone-${tone(n.score,best)}`;
 const delayColor=n.checks&&!n.last_success?'bad':delayTone(n.delay_ms);
 if(n.delay_ms!=null)animateNumber(row.cells[3].firstElementChild,n.delay_ms);else{row.cells[3].firstElementChild.textContent=n.checks?'失败':'—';delete row.cells[3].firstElementChild.dataset.value;}
 row.cells[3].firstElementChild.className=`metric-cell tone-${delayColor}`;
 const nextHistoryKey=historyKey(n);
 const historyChanged=row.dataset.historyKey!==nextHistoryKey;
 if(historyChanged)queueHistory(row,n.history,best);
 else if(!row._historyPlaying){
  row.cells[4].innerHTML=history(n.history,'score',best,false);
  row.cells[5].innerHTML=history(n.history,'delay',best,false);
 }
 row.dataset.historyKey=nextHistoryKey;
 row.dataset.renderKey=nodeKey(n,best,s,selected);
}
function syncRows(body,nodes,best,s,selected=false) {
 if(typeof body.querySelectorAll!=='function') {
  body.innerHTML=selected?nodeRow(nodes[0],best,s):nodes.map(n=>nodeRow(n,best,s)).join('');
  return;
 }
 const rows=new Map([...body.querySelectorAll('tr[data-row-id]')].map(row=>[row.dataset.rowId,row]));
 const seen=new Set();
 let position=body.firstElementChild;
 for(const n of nodes) {
  const id=n.id||'selected';
  let row=rows.get(id);
  const key=nodeKey(n,best,s,selected);
  if(!row) {
   const holder=document.createElement('tbody');
   holder.innerHTML=nodeRow(n,best,s);
   row=holder.firstElementChild;
   row.dataset.renderKey=key;
   row.dataset.historyKey=historyKey(n);
   row._paintedHistory=n.history??[];
  }
  if(row.dataset.renderKey!==key)patchRow(row,n,best,s,selected);
  if(row!==position)body.insertBefore(row,position);
  position=row.nextElementSibling;
  seen.add(id);
 }
 for(const [id,row] of rows)if(!seen.has(id))row.remove();
 for(const row of [...body.children])if(!row.dataset.rowId)row.remove();
}
function render(s){
 if(s.instance_id&&stateInstance&&s.instance_id===stateInstance&&Number.isFinite(Number(s.revision))&&Number(s.revision)<stateRevision)return;
 if(s.instance_id)stateInstance=s.instance_id;
 if(Number.isFinite(Number(s.revision)))stateRevision=Number(s.revision);
 state=s;
 if(typeof syncLongTermOverview==='function')syncLongTermOverview(s);
 if(typeof SmartSettings!=='undefined')SmartSettings.update(s.settings);
 const prefs=s.settings;
 if(prefs?.delay_green_ms){
 if($('legend-green'))$('legend-green').textContent=`延迟 ≤${prefs.delay_green_ms}ms`;
 if($('legend-yellow'))$('legend-yellow').textContent=`${prefs.delay_green_ms}–${prefs.delay_yellow_ms}ms`;
 if($('legend-red'))$('legend-red').textContent=`>${prefs.delay_yellow_ms}ms`;
 if($('switch-rule'))$('switch-rule').textContent=`${prefs.switch_ratio} 倍时切换`;
 if($('frequency-note'))$('frequency-note').textContent=`当前 ${prefs.current_seconds}s · 候选 ${prefs.candidate_seconds}s · 普通 ${prefs.ordinary_seconds}s · 恢复复查逐步放慢`;
 }
 $('detection-toggle').textContent=(s.paused||s.sleeping)?'继续检测':'暂停检测';
 $('detection-toggle').disabled=busy;
 $('recheck').disabled=busy||s.paused||s.sleeping;
 $('selection-auto').disabled=busy||Boolean(s.pending_id)||s.selection_mode!=='manual';
 $('selection-auto').setAttribute?.('aria-pressed',s.selection_mode!=='manual');
 $('selection-manual').disabled=busy||s.selection_mode==='manual';
 $('selection-manual').setAttribute?.('aria-pressed',s.selection_mode==='manual');
 $('selection-auto').classList.toggle('active',s.selection_mode!=='manual');
 $('selection-manual').classList.toggle('active',s.selection_mode==='manual');
 const apiInput=$('api-address');
 if(apiInput && apiInput.dataset && !apiInput.dataset.loaded){apiInput.value=s.api_address||'';apiInput.dataset.loaded='yes';}
 if($('group-root') && $('group-root').dataset && !$('group-root').dataset.loaded){$('group-root').value=s.group_root||'proxy';$('group-pattern').value=s.group_pattern||'PIN$';$('group-root').dataset.loaded='yes';}
 $('connection').parentElement.dataset.health=s.api_healthy?'ok':'error';
 $('connection').textContent=s.api_healthy?'后端已连接':'管理接口异常';
 $('mode-note-text').textContent=s.mode==='real'?'真实模式 · 已连接 sing-box API':'模拟模式 · 切换仅作用于模拟接口';
 $('mode-note').classList.toggle('real-mode',s.mode==='real');
 const nextAirportKey=JSON.stringify({airports:s.airports,selected:s.airport_id,busy,pending:s.pending_id,paused:s.paused,sleeping:s.sleeping,active:s.control_active});
 if(nextAirportKey!==airportKey){
 $('airport-options').innerHTML=s.airports.map(a=>`<button class="airport-option ${a.id===s.airport_id&&s.control_active!==false?'selected':''}" data-airport="${esc(a.id)}" title="${esc(a.selector)}" ${busy||s.paused||s.sleeping||s.pending_id||(a.id===s.airport_id&&s.control_active!==false)?'disabled':''} aria-pressed="${a.id===s.airport_id&&s.control_active!==false}"><strong>${esc(a.selector)}</strong></button>`).join('');
  airportKey=nextAirportKey;
 }
 const available=s.nodes.filter(n=>n.checks>0&&n.last_success).length;
 $('success-summary').textContent=available+' / '+s.nodes.length+' 节点成功';
 $('sample-count').textContent=s.nodes.reduce((v,n)=>v+n.checks,0)+' 次检查';
 const current=s.nodes.find(n=>n.id===s.current_id);
 const elapsed=(new Date(s.now)-new Date(s.started_at))/1000;
 const observationSeconds=s.observation_seconds||s.startup_seconds;
 $('status-line').textContent=s.paused?'检测已暂停，分数和历史保留。':s.sleeping?'长时间低流量，已自动暂停检测；达到唤醒流量阈值后自动恢复。':!s.api_healthy?'管理接口不可用：暂停选择，不将接口错误记为节点失败。':s.control_active===false?`根策略组当前选择 ${s.root_selection||'尚未同步'}，smart 暂不自动切换。`:s.pending_id?'正在确认切换目标，读取接口结果后更新当前选择。':s.phase==='startup'?`启动/恢复观察中：${Math.min(elapsed,observationSeconds).toFixed(1)} / ${observationSeconds} 秒 · 观察结束后开始自动判断。`:s.phase==='unavailable'?`暂时全部不可用：每 ${s.settings?.retry_seconds??3} 秒重新检查，保留已有选择。`:s.selection_mode==='manual'?'手动选择模式：继续检测并更新数据，smart 不会自动切换节点。':'';
 const valid=s.nodes.filter(n=>n.score!=null), best=valid.length?Math.min(...valid.map(n=>n.score)):Infinity;
 const sorted=[...s.nodes].sort((a,b)=>(a.score??Infinity)-(b.score??Infinity)||a.id.localeCompare(b.id));
 const selected=current??{id:'',name:'等待首次选择',checks:0,history:[],score:null,delay_ms:null,last_success:false};
 syncRows($('selected-node'),[selected],best,s,true);
 nodeList??=SmartNodeList.create(30);
 const list=nodeList.view(sorted,s.airport_id,prefs?.node_display_limit??30);
 $('node-count').textContent=list.canToggle?`${list.visible} / ${list.total}`:list.total;
 $('node-list-controls').hidden=!list.canToggle;
 $('nodes-toggle').textContent=list.expanded?`收起至前 ${list.limit} 个`:`显示全部（还有 ${list.remaining} 个）`;
 $('nodes-toggle').setAttribute('aria-expanded',String(list.expanded));
 syncRows($('nodes'),list.nodes,best,s,false);
 const kinds={startup:'启动',phase:'状态',switch:'切换',recovery:'复查',sleep:'睡眠',manual:'手动',decision:'判断',api:'接口',error:'错误',unavailable:'故障',grpc:'gRPC',settings:'设置'};
 const nextEventsKey=JSON.stringify(s.events);
 if(nextEventsKey!==eventsKey){
  $('events').innerHTML=[...s.events].reverse().slice(0,8).map(e=>{const kind=kinds[e.module||e.kind]??'记录';const eventNames={connected:'已连接',disconnected:'已断开',connect_failed:'连接失败',connect:'连接中',entered:'进入睡眠',woke:'已唤醒',applied:'已应用'};const event=e.event&&e.event!=='state'?` · ${eventNames[e.event]||e.event}`:'';const level=['DEBUG','INFO','WARN','ERROR'].includes(e.level)?e.level:'INFO';const cls=(e.module||e.kind)==='sleep'?' sleep-event':'';return `<div class="event${cls}"><time>${recordTime(e.at)}</time><span class="event-level level-${level.toLowerCase()}">${level}</span><span class="event-kind">${esc(kind)}</span><span class="event-event">${esc(event)}</span><span class="event-text" title="${esc(e.message)}">${esc(e.message)}</span></div>`;}).join('');
  eventsKey=nextEventsKey;
 }
 const sleepRecords=s.sleep_records??[];
 const nextSleepRecordsKey=JSON.stringify(sleepRecords);
 if(nextSleepRecordsKey!==sleepRecordsKey){
  $('sleep-count').textContent=sleepRecords.length;
  $('sleep-records').innerHTML=sleepRecords.length?[...sleepRecords].reverse().map(record=>{
   const elapsed=elapsedTime(record.duration_seconds);
   const range=record.active?`${recordTime(record.started_at)} 开始`:`${recordTime(record.started_at)} — ${recordTime(record.ended_at)}`;
   const wake=record.wake_source?` · 唤醒：${record.wake_source}`:record.wake_reason==='settings'?' · 关闭自动睡眠':record.wake_reason==='manual'?' · 手动继续':'';
   const label=record.active?`正在睡眠 · 已持续 ${elapsed}`:`持续 ${elapsed}${wake}`;
   return `<div class="sleep-record"><span class="sleep-record-time">${esc(range)}</span><span class="sleep-record-state">${esc(label)}</span></div>`;
  }).join(''):'<div class="sleep-record-empty">暂无睡眠记录</div>';
  sleepRecordsKey=nextSleepRecordsKey;
 }
 $('updated').textContent='更新于 '+time(s.now);
}
function toast(message){$('toast').textContent=message;$('toast').hidden=false;clearTimeout(toastTimer);toastTimer=setTimeout(()=>$('toast').hidden=true,4500);}
async function refresh(){try{const headers=stateEtag?{'If-None-Match':stateEtag}:{};const r=await fetch('/api/state',{headers});if(r.status===304)return;if(!r.ok)throw new Error('状态读取失败');stateEtag=r.headers.get('ETag')||'';render(await r.json());}catch(e){$('connection').parentElement.dataset.health='error';$('connection').textContent='连接中断';$('status-line').textContent='暂时无法连接后端，当前显示的是上次数据。';}}
async function control(action,id=''){
 if(busy)return;busy=true;if(state)render(state);
 try{const r=await fetch('/api/control/'+action,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id})});const data=await r.json();if(!r.ok)throw new Error(data.error||'操作失败');toast(action==='recheck'?'已安排当前策略组节点检查':action==='pause'?'检测已暂停':action==='resume'?'已继续检测':action==='auto'?'已开启自动选择':action==='manual'?'已开启手动选择':'请求已受理，等待后端确认。');}
 catch(e){toast(e.message);}finally{busy=false;await refresh();}
}
$('airport-options').addEventListener('click',e=>{const button=e.target.closest('[data-airport]');if(button)control('airport',button.dataset.airport);});
$('recheck').addEventListener('click',()=>control('recheck'));
$('detection-toggle').addEventListener('click',()=>{if(state)control((state.paused||state.sleeping)?'resume':'pause');});
$('selection-auto').addEventListener('click',()=>control('auto'));
$('selection-manual').addEventListener('click',()=>control('manual'));
if($('api-form'))$('api-form').addEventListener('submit',async e=>{e.preventDefault();const api=$('api-address').value.trim();if(!api)return toast('请输入 API 地址');try{const r=await fetch('/api/config',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({api,root:$('group-root').value.trim(),pattern:$('group-pattern').value.trim()})});const data=await r.json();if(!r.ok)throw new Error(data.error||'设置应用失败');toast('设置已应用，正在重新读取节点');await refresh();}catch(err){toast(err.message);}});
for (const id of ['nodes','selected-node']) {
 const body=$(id);
 body.closest('table').insertAdjacentHTML('afterbegin',tableHeading());
 body.addEventListener('click',e=>{
  const detail=e.target.closest('button[data-detail-node]');
  if(detail&&!detail.disabled){e.stopPropagation();openNodeDetail(detail.dataset.detailNode);return;}
  const button=e.target.closest('button[data-node]');
  if(button&&!button.disabled)control('node',button.dataset.node);
 });
}
// Background tabs do not need a full 78-node snapshot every second.
async function poll(){if(!streamOpen||Date.now()-lastStreamEvent>3000)await refresh();setTimeout(poll,document.hidden?5000:1000);}poll();
document.addEventListener('visibilitychange',()=>{if(!document.hidden)refresh();});

function connectStream(){
 if(!window.EventSource)return;
 stream=new EventSource('/api/events');
 stream.addEventListener('state',event=>{try{
  stateEtag='';lastStreamEvent=Date.now();
  const message=JSON.parse(event.data);
  if(message.full||!state){render(message);}
  else {
   const byId=new Map(state.nodes.map(node=>[node.id,node]));
   for(const id of message.removed??[])byId.delete(id);
   for(const node of message.nodes??[])byId.set(node.id,node);
   render({...state,...message,nodes:[...byId.values()]});
  }
  streamOpen=true;
 }catch(_){}});
 stream.onopen=()=>{streamOpen=true;refresh();};
 stream.onerror=()=>{streamOpen=false;stream.close();setTimeout(connectStream,3000);};
}
connectStream();

$('api-test').addEventListener('click',async()=>{const button=$('api-test');button.disabled=true;try{const r=await fetch('/api/config/api/test',{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({api:$('api-address').value.trim()})});const data=await r.json();if(!r.ok)throw new Error(data.error);toast('连接正常，策略组可读取');}catch(e){toast(e.message);}finally{button.disabled=false;}});

$('nodes-toggle').addEventListener('click',()=>{if(!state||!nodeList)return;nodeList.toggle();render(state);});
$('node-detail-close')?.addEventListener('click',closeNodeDetail);
$('node-detail-dialog')?.addEventListener('close',()=>{if(detailNodeId)closeNodeDetail();else resetDetailView();});
$('detail-view-realtime')?.addEventListener('click',()=>setDetailView('realtime'));
$('detail-view-longterm')?.addEventListener('click',()=>setDetailView('longterm'));
