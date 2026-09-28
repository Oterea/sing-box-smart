const $ = id => document.getElementById(id);
let state, airportDirty = false, busy = false, toastTimer;
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
  const x=4+index*16;
  const title=time(s.at)+' · '+(kind==='score'?'分数 '+num(value):s.success?'实际延迟 '+num(value)+' ms':'失败：'+s.error)+(kind==='score'&&!s.success?' · 本次探测失败':'');
  let shape;
  if(kind==='delay'&&!s.success){
   // Failure has no latency value; an outlined marker distinguishes it from a measured bar.
   shape=`<rect class="chart-failure" x="${x}" y="3" width="9" height="47" rx="1"/>`;
  } else if(!Number.isFinite(value)){
   shape=`<path class="chart-missing" d="M${x} 49h9"/>`;
  } else {
   const height=Math.max(1,value/ceiling*47);
   const color=kind==='score'?tone(value,best):value<=200?'good':value<=400?'medium':'bad';
   shape=`<rect class="chart-bar bar-${color}" x="${x}" y="${50-height}" width="9" height="${height}" rx="1"/>`;
  }
  return `<g><title>${esc(title)}</title>${shape}</g>`;
 }).join('');
 return `<svg class="history-chart" viewBox="0 0 320 52" preserveAspectRatio="none" role="img" aria-label="${kind==='score'?'分数':'延迟'}历史，最近${tail.length}次，旧到新，纵轴上限${Math.round(ceiling)}"><path class="chart-grid" d="M0 3H320M0 26.5H320"/><path class="chart-baseline" d="M0 50.5H320"/>${bars}${tail.length?'':'<text x="4" y="33" fill="#95a2ad" font-size="12">暂无记录</text>'}</svg>`;
}
function render(s){
 state=s;
 $('connection').parentElement.dataset.health=s.api_healthy?'ok':'error';
 $('connection').textContent=s.api_healthy?'后端已连接':'管理接口异常';
 $('mode-note-text').textContent=s.mode==='real'?'真实模式 · 已连接 sing-box API':'模拟模式 · 切换仅作用于模拟接口';
 $('mode-note').classList.toggle('real-mode',s.mode==='real');
 const active=s.airports.find(a=>a.id===s.airport_id);
 if(!$('airport').options.length){$('airport').innerHTML=s.airports.map(a=>`<option value="${esc(a.id)}">${esc(a.name)}</option>`).join('');}
 if(!airportDirty)$('airport').value=s.airport_id;
 $('selector-name').textContent='当前生效：'+active.selector;
 $('switch-airport').disabled=busy||Boolean(s.pending_id)||$('airport').value===s.airport_id;
 $('node-count').textContent=s.nodes.length;
 $('scope').textContent=active.name+' · '+active.selector+' · 仅显示本机场节点';
 $('available').textContent=s.nodes.filter(n=>n.checks>0&&n.last_success).length;
 $('total').textContent=' / '+s.nodes.length+' 节点成功';
 const failed=s.nodes.filter(n=>n.checks>0&&!n.last_success).length;
 $('failed').textContent=failed?failed+' 个节点最近检查失败':'最近检查均正常';
 $('sample-count').textContent=s.nodes.reduce((v,n)=>v+n.checks,0)+' 次检查';
 $('health-progress').value=100*Number($('available').textContent)/(s.nodes.length||1);
 const current=s.nodes.find(n=>n.id===s.current_id);
 $('current-name').textContent=current?.name??'等待首次选择';
 $('current-detail').textContent=current?active.selector+' · '+(current.last_success?'最近检查成功':'最近检查失败，正在重新比较'):'先观察约 10 秒，再根据分数选择节点。';
 $('current-score').textContent=current?.score_overflow?'∞':num(current?.score);
 $('current-delay').textContent=current?.delay_ms!=null?num(current.delay_ms):current?.checks?'失败':'—';
 $('phase').textContent=!s.api_healthy?'接口异常':s.phase==='startup'?'启动检查':s.phase==='unavailable'?'等待恢复':'自动选择';
 const elapsed=(new Date(s.now)-new Date(s.started_at))/1000;
 $('status-line').textContent=!s.api_healthy?'管理接口不可用：暂停选择，不将接口错误记为节点失败。':s.phase==='startup'?`启动检查 ${Math.min(elapsed,s.startup_seconds).toFixed(1)} / ${s.startup_seconds} 秒 · 各节点独立连续检查，观察结束后首次选择。`:s.pending_id?'正在确认切换目标，读取接口结果后更新当前选择。':s.phase==='unavailable'?'暂时全部不可用：每 3 秒重新检查，保留已有选择。':'自动选择已开启 · 当前分数超过最好候选的 1.4 倍时，先确认候选，再切换。';
 const valid=s.nodes.filter(n=>n.score!=null), best=Math.min(...valid.map(n=>n.score));
 const sorted=[...s.nodes].sort((a,b)=>(a.score??Infinity)-(b.score??Infinity)||a.id.localeCompare(b.id));
 $('nodes').innerHTML=sorted.map((n,i)=>{const delayTone=n.checks&&!n.last_success?'bad':n.delay_ms==null?'none':n.last_success?(n.delay_ms<=200?'good':n.delay_ms<=400?'medium':'bad'):'bad';return `<tr class="${n.current?'current-row':''}"><td class="rank">${String(i+1).padStart(2,'0')}</td><td><div class="node-name">${esc(n.name)}</div><div class="node-meta">${n.current?'<span class="badge">使用中</span>':''}<span>${esc(n.frequency)}</span><span>· ${n.checks} 次</span></div></td><td><div class="metric-pair"><span class="score-value tone-${tone(n.score,best)}">${n.score_overflow?'∞':num(n.score)}</span><span class="delay-value tone-${delayTone}">${n.delay_ms!=null?num(n.delay_ms):n.checks?'失败':'—'}</span></div></td><td><div class="history-pair">${history(n.history,'score',best)}${history(n.history,'delay',best)}</div></td><td>${n.current?'<span class="current-label">当前选中</span>':`<button class="pick" data-node="${esc(n.id)}" ${busy||s.pending_id||s.phase==='startup'?'disabled':''}>选择</button>`}</td></tr>`}).join('');
 const kinds={startup:'启动',phase:'状态',switch:'切换',recovery:'复查',manual:'手动',decision:'判断',api:'接口',error:'错误',unavailable:'故障'};
 $('events').innerHTML=[...s.events].reverse().slice(0,8).map(e=>`<div class="event"><time>${time(e.at)}</time><span class="event-kind">${kinds[e.kind]??'记录'}</span><span class="event-text" title="${esc(e.message)}">${esc(e.message)}</span></div>`).join('');
 $('updated').textContent='更新于 '+time(s.now);
}
function toast(message){$('toast').textContent=message;$('toast').hidden=false;clearTimeout(toastTimer);toastTimer=setTimeout(()=>$('toast').hidden=true,4500);}
async function refresh(){try{const r=await fetch('/api/state');if(!r.ok)throw new Error('状态读取失败');render(await r.json());}catch(e){$('connection').parentElement.dataset.health='error';$('connection').textContent='连接中断';$('status-line').textContent='暂时无法连接后端，当前显示的是上次数据。';}}
async function control(action,id=''){
 if(busy)return;busy=true;if(state)render(state);
 try{const r=await fetch('/api/control/'+action,{method:'POST',headers:{'Content-Type':'application/json'},body:JSON.stringify({id})});const data=await r.json();if(!r.ok)throw new Error(data.error||'操作失败');if(action==='airport')airportDirty=false;toast(action==='recheck'?'已安排全部节点检查':'请求已受理，等待后端确认。');}
 catch(e){toast(e.message);}finally{busy=false;await refresh();}
}
$('airport').addEventListener('change',()=>{airportDirty=true;if(state)render(state);});
$('switch-airport').addEventListener('click',()=>control('airport',$('airport').value));
$('recheck').addEventListener('click',()=>control('recheck'));
$('nodes').addEventListener('click',e=>{const button=e.target.closest('[data-node]');if(button)control('node',button.dataset.node);});
async function poll(){await refresh();setTimeout(poll,1000);}poll();
