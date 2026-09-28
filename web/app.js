const $ = id => document.getElementById(id);
let state, busy = false, toastTimer;
const esc = value => String(value ?? '').replace(/[&<>"']/g, c => ({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));
const num = value => value == null ? '—' : value >= 10000 ? (value / 1000).toFixed(1) + 'k' : Math.round(value).toString();
const time = value => new Date(value).toLocaleTimeString('zh-CN', {hour12:false});
const tone = (value,best) => value == null ? 'none' : value <= best*1.4 ? 'good' : value <= best*2.5 ? 'medium' : 'bad';
function history(samples, kind, best) {
 const tail=(samples ?? []).slice(-20);
 const values=tail.map(s=>kind==='score'?s.score:s.delay_ms).filter(v=>Number.isFinite(v)&&v>=0);
 const ceiling=Math.max(kind==='delay'?400:1,...values);
 // SVG geometry attributes work with the server's strict CSP (no inline styles).
 const bars=tail.map((s,index)=>{
  const value=kind==='score'?s.score:s.delay_ms;
  const x=6+index*15;
  const title=time(s.at)+' · '+(kind==='score'?'分数 '+num(value):s.success?'实际延迟 '+num(value)+' ms':'失败：'+s.error)+(kind==='score'&&!s.success?' · 本次探测失败':'');
  let shape;
  if(kind==='delay'&&!s.success){
   // Failure has no latency value; an outlined marker distinguishes it from a measured bar.
   shape=`<rect class="chart-failure" x="${x}" y="2" width="4" height="32"/>`;
  } else if(!Number.isFinite(value)){
   shape=`<path class="chart-missing" d="M${x} 33h5"/>`;
  } else {
   const height=Math.max(1,value/ceiling*31);
   const color=kind==='score'?tone(value,best):value<=200?'good':value<=400?'medium':'bad';
   shape=`<rect class="chart-bar bar-${color}" x="${x}" y="${33-height}" width="4" height="${height}"/>`;
  }
  return `<g><title>${esc(title)}</title>${shape}</g>`;
 }).join('');
 return `<svg class="history-chart" viewBox="0 0 320 36" preserveAspectRatio="none" role="img" aria-label="${kind==='score'?'分数':'延迟'}历史，最近${tail.length}次，旧到新，纵轴上限${Math.round(ceiling)}"><path class="chart-grid" d="M0 2H320M0 18H320"/><path class="chart-baseline" d="M0 33.5H320"/>${bars}${tail.length?'':'<text x="4" y="23" fill="#95a2ad" font-size="12">暂无记录</text>'}</svg>`;
}
// Both sections share the same header, column definitions and row renderer.
function tableHeading() {
 return '<colgroup><col class="col-node"><col class="col-action"><col class="col-score"><col class="col-delay"><col class="col-history"><col class="col-history"></colgroup><thead><tr><th scope="col">节点</th><th scope="col">选择</th><th scope="col">分数</th><th scope="col">延迟（ms）</th><th scope="col">分数历史</th><th scope="col">延迟历史</th></tr></thead>';
}
function nodeRow(n,best,s) {
 const current=Boolean(n.id)&&n.id===s.current_id;
 const disabled=busy||Boolean(s.pending_id)||s.phase==='startup'||!s.api_healthy||current||!n.id;
 const delayTone=n.checks&&!n.last_success?'bad':n.delay_ms==null?'none':n.delay_ms<=200?'good':n.delay_ms<=400?'medium':'bad';
 return `<tr class="${current?'current-row':''}"><td><div class="node-cell"><div class="node-name">${esc(n.name)}</div><div class="node-meta">${n.checks} 次检查</div></div></td><td><button class="pick${current?' current-pick':''}" data-node="${esc(n.id)}" ${disabled?'disabled':''}>${current?'已选中':!n.id?'等待中':'选择'}</button></td><td><span class="metric-cell tone-${tone(n.score,best)}">${n.score_overflow?'∞':num(n.score)}</span></td><td><span class="metric-cell tone-${delayTone}">${n.delay_ms!=null?num(n.delay_ms):n.checks?'失败':'—'}</span></td><td>${history(n.history,'score',best)}</td><td>${history(n.history,'delay',best)}</td></tr>`;
}
function render(s){
 state=s;
 $('connection').parentElement.dataset.health=s.api_healthy?'ok':'error';
 $('connection').textContent=s.api_healthy?'后端已连接':'管理接口异常';
 $('mode-note-text').textContent=s.mode==='real'?'真实模式 · 已连接 sing-box API':'模拟模式 · 切换仅作用于模拟接口';
 $('mode-note').classList.toggle('real-mode',s.mode==='real');
 const active=s.airports.find(a=>a.id===s.airport_id)??s.airports[0];
 $('airport-options').innerHTML=s.airports.map(a=>`<button class="airport-option ${a.id===s.airport_id?'selected':''}" data-airport="${esc(a.id)}" ${busy||s.pending_id||a.id===s.airport_id?'disabled':''} aria-pressed="${a.id===s.airport_id}"><strong>${esc(a.name)}</strong><small>${esc(a.selector)}</small></button>`).join('');
 $('selector-name').textContent=active?'当前生效：'+active.selector:'暂无机场';
 $('node-count').textContent=s.nodes.length;
 $('available').textContent=s.nodes.filter(n=>n.checks>0&&n.last_success).length;
 $('total').textContent=' / '+s.nodes.length+' 节点成功';
 const failed=s.nodes.filter(n=>n.checks>0&&!n.last_success).length;
 $('failed').textContent=failed?failed+' 个节点最近检查失败':'最近检查均正常';
 $('sample-count').textContent=s.nodes.reduce((v,n)=>v+n.checks,0)+' 次检查';
 $('health-progress').value=100*Number($('available').textContent)/(s.nodes.length||1);
 const current=s.nodes.find(n=>n.id===s.current_id);
 $('phase').textContent=!s.api_healthy?'接口异常':s.phase==='startup'?'启动检查':s.phase==='unavailable'?'等待恢复':'自动选择';
 const elapsed=(new Date(s.now)-new Date(s.started_at))/1000;
 $('status-line').textContent=!s.api_healthy?'管理接口不可用：暂停选择，不将接口错误记为节点失败。':s.phase==='startup'?`启动检查 ${Math.min(elapsed,s.startup_seconds).toFixed(1)} / ${s.startup_seconds} 秒 · 各节点独立连续检查，观察结束后首次选择。`:s.pending_id?'正在确认切换目标，读取接口结果后更新当前选择。':s.phase==='unavailable'?'暂时全部不可用：每 3 秒重新检查，保留已有选择。':'';
 const valid=s.nodes.filter(n=>n.score!=null), best=valid.length?Math.min(...valid.map(n=>n.score)):Infinity;
 const sorted=[...s.nodes].sort((a,b)=>(a.score??Infinity)-(b.score??Infinity)||a.id.localeCompare(b.id));
 const selected=current??{id:'',name:'等待首次选择',checks:0,history:[],score:null,delay_ms:null,last_success:false};
 $('selected-node').innerHTML=nodeRow(selected,best,s);
 $('nodes').innerHTML=sorted.length?sorted.map(n=>nodeRow(n,best,s)).join(''):'<tr><td colspan="6" class="empty">暂无节点</td></tr>';
 const kinds={startup:'启动',phase:'状态',switch:'切换',recovery:'复查',manual:'手动',decision:'判断',api:'接口',error:'错误',unavailable:'故障'};
 $('events').innerHTML=[...s.events].reverse().slice(0,8).map(e=>`<div class="event"><time>${time(e.at)}</time><span class="event-kind">${kinds[e.kind]??'记录'}</span><span class="event-text" title="${esc(e.message)}">${esc(e.message)}</span></div>`).join('');
 $('updated').textContent='更新于 '+time(s.now);
}
function toast(message){$('toast').textContent=message;$('toast').hidden=false;clearTimeout(toastTimer);toastTimer=setTimeout(()=>$('toast').hidden=true,4500);}
async function refresh(){try{const r=await fetch('/api/state');if(!r.ok)throw new Error('状态读取失败');render(await r.json());}catch(e){$('connection').parentElement.dataset.health='error';$('connection').textContent='连接中断';$('status-line').textContent='暂时无法连接后端，当前显示的是上次数据。';}}
async function control(action,id=''){
 if(busy)return;busy=true;if(state)render(state);
 try{const r=await fetch('/api/control/'+action,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id})});const data=await r.json();if(!r.ok)throw new Error(data.error||'操作失败');toast(action==='recheck'?'已安排全部节点检查':'请求已受理，等待后端确认。');}
 catch(e){toast(e.message);}finally{busy=false;await refresh();}
}
$('airport-options').addEventListener('click',e=>{const button=e.target.closest('[data-airport]');if(button)control('airport',button.dataset.airport);});
$('recheck').addEventListener('click',()=>control('recheck'));
for (const id of ['nodes','selected-node']) {
 const body=$(id);
 body.closest('table').insertAdjacentHTML('afterbegin',tableHeading());
 body.addEventListener('click',e=>{
  const button=e.target.closest('button[data-node]');
  if(button&&!button.disabled)control('node',button.dataset.node);
 });
}
async function poll(){await refresh();setTimeout(poll,1000);}poll();
