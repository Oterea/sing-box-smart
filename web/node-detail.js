function stabilityLabel(summary) {
 return stabilityConclusion(summary,Number(state?.settings?.delay_green_ms??200),Number(state?.settings?.delay_yellow_ms??600));
}
function detailMetric(id,label,value) { return `<div class="detail-metric"><span>${esc(label)}</span><strong id="${id}">${esc(value)}</strong></div>`; }
function paintDetailChart(scroll, samples) {
 const track=scroll.querySelector('.stability-chart-track'), layer=scroll.querySelector('.stability-chart-layer');
 if(!track||!layer)return;
 const slot=8, bar=4, ceiling=1000;
 const totalWidth=Math.max(scroll.clientWidth,samples.length*slot);
 layer.setAttribute('width',String(totalWidth));
 layer.setAttribute('viewBox',`0 0 ${totalWidth} 76`);
 const draw=()=>{
  const first=Math.max(0,Math.floor(scroll.scrollLeft/slot)-120), count=Math.ceil(scroll.clientWidth/slot)+240, last=Math.min(samples.length,first+count);
  layer.innerHTML=samples.slice(first,last).map((sample,index)=>{
   const value=Number(sample.delay_ms), height=sample.success&&Number.isFinite(value)?Math.max(2,Math.min(56,value/ceiling*56)):56;
   const color=!sample.success?'failure':delayTone(value);
   const title=recordTime(sample.at)+' · '+(sample.success?'实际延迟 '+Math.round(value)+' ms':'失败'+(sample.error?'：'+sample.error:''));
   const x=(first+index)*slot, y=64-height;
   return `<g><title>${esc(title)}</title><rect class="stability-bar bar-${color}" x="${x}" y="${y}" width="${bar}" height="${height}"/></g>`;
  }).join('');
 };
 scroll._detailDraw=draw;
 if(!scroll._detailBound){
  const stopAutoFollow=()=>{detailFollowLatest=false;scroll.scrollLeft=scroll.scrollLeft;};
  scroll.addEventListener('scroll',()=>{const max=Math.max(0,scroll.scrollWidth-scroll.clientWidth);detailFollowLatest=max-scroll.scrollLeft<=32;requestAnimationFrame(()=>scroll._detailDraw?.());});
  scroll.addEventListener('wheel',stopAutoFollow,{passive:true});
  scroll.addEventListener('touchstart',stopAutoFollow,{passive:true});
  scroll.addEventListener('pointerdown',stopAutoFollow,{passive:true});
  scroll.addEventListener('keydown',event=>{if(['ArrowLeft','ArrowRight','PageUp','PageDown','Home','End'].includes(event.key))stopAutoFollow();});
  scroll._detailBound=true;
 }
 draw();
}
function updateDetailSummary(summary) {
 const [label,toneName]=stabilityLabel(summary);
 const result=$('detail-stability-result');
 if(result){result.className=`stability-result tone-${toneName}`;$('detail-stability-label').textContent=label;}
 const values={
  'detail-p50':detailMs(summary.p50_ms),
  'detail-p90':detailMs(summary.p90_ms),
  'detail-success-rate':detailPercent(summary.success_rate),
  'detail-high-latency':detailPercent(summary.high_latency_rate),
  'detail-max-failures':String(summary.max_consecutive_failures||0)+' 次',
  'detail-samples':String(summary.samples||0)+' 次'
 };
 for(const [id,value] of Object.entries(values)){const element=$(id);if(element)element.textContent=value;}
 $('node-detail-subtitle').textContent='最近 24 小时 · '+(summary.started_at?recordTime(summary.started_at):'尚无记录');
}
function updateDetailChart(chart, samples, preserveScroll, followLatest) {
 const previousScrollLeft=chart.scrollLeft;
 paintDetailChart(chart,samples);
 requestAnimationFrame(()=>{
  const maxScroll=Math.max(0,chart.scrollWidth-chart.clientWidth);
  if(preserveScroll&&followLatest&&maxScroll>previousScrollLeft)chart.scrollTo({left:maxScroll,behavior:'smooth'});
  else chart.scrollLeft=preserveScroll?Math.min(previousScrollLeft,maxScroll):maxScroll;
 });
}
function longTermSummaryText(summary) {
 return stabilityConclusion(summary,Number(state?.settings?.delay_green_ms??200),Number(state?.settings?.delay_yellow_ms??600))[0];
}
function renderLongTerm(data) {
 const content=$('node-detail-content');
 if(!content)return;
 const days=(data?.days||[]).slice().sort((a,b)=>a.date.localeCompare(b.date)).reverse();
 if(!days.length){content.innerHTML='<div class="detail-empty">暂无 14 天聚合数据</div>';return;}
 const thresholds={green:Number(state?.settings?.delay_green_ms??200),yellow:Number(state?.settings?.delay_yellow_ms??600)};
 content.innerHTML=days.map(day=>`<section class="longterm-day"><div class="longterm-day-heading"><strong>${esc(day.date)}</strong><span class="subtle">${esc(longTermSummaryText(day.summary))} · P50 ${esc(detailMs(day.summary?.p50_ms))} · P90 ${esc(detailMs(day.summary?.p90_ms))}</span></div>${longTermChart(day,thresholds)}</section>`).join('');
}
async function loadLongTerm() {
 if(!detailNodeId)return;
 const nodeID=detailNodeId;
 detailLongTermController?.abort();
 const controller=new AbortController();
 detailLongTermController=controller;
 try {
  const response=await fetch('/api/node/'+encodeURIComponent(nodeID)+'/stability/long-term',{cache:'no-store',signal:controller.signal});
  if(!response.ok)throw new Error('长期趋势读取失败');
  const data=await response.json();
  if(detailNodeId!==nodeID)return;
  if(detailView==='longterm')renderLongTerm(data);
 } catch(error) {
  if(error.name!=='AbortError'&&detailNodeId===nodeID)$('node-detail-content').innerHTML=`<div class="detail-empty">${esc(error.message)}</div>`;
 } finally { if(detailLongTermController===controller)detailLongTermController=null; }
}
function setDetailView(view) {
 detailView=view;
 const realtime=$('detail-view-realtime'), longterm=$('detail-view-longterm');
 if(realtime){realtime.classList.toggle('active',view==='realtime');realtime.setAttribute('aria-selected',String(view==='realtime'));}
 if(longterm){longterm.classList.toggle('active',view==='longterm');longterm.setAttribute('aria-selected',String(view==='longterm'));}
 if(view==='realtime'){detailLongTermController?.abort();renderDetail(detailData);$('node-detail-subtitle').textContent='最近 24 小时';}
 else { $('node-detail-subtitle').textContent='最近 14 天 · 每 15 分钟聚合'; loadLongTerm(); }
}
function resetDetailView() {
 detailView='realtime';
 const realtime=$('detail-view-realtime'), longterm=$('detail-view-longterm');
 if(realtime){realtime.classList.add('active');realtime.setAttribute('aria-selected','true');}
 if(longterm){longterm.classList.remove('active');longterm.setAttribute('aria-selected','false');}
 $('node-detail-subtitle').textContent='最近 24 小时';
}
function renderDetail(data) {
 const content=$('node-detail-content');
 if(!content||!data)return;
 if(detailView==='longterm')return;
 const summary=data.summary||{}, samples=data.samples||[], [label,toneName]=stabilityLabel(summary);
 const summaryKey=stabilitySummaryKey(summary), samplesKey=stabilitySamplesKey(samples);
 const summaryChanged=detailSummaryKey!==summaryKey;
 const samplesChanged=detailSamplesKey!==samplesKey;
 if(!summaryChanged&&!samplesChanged&&content.querySelector('.stability-chart-scroll'))return;
 const node=state?.nodes?.find(n=>n.id===detailNodeId);
 const name=node?.name||detailNodeId;
 $('node-detail-title').textContent=name+' · 详情';
 const existingChart=content.querySelector('.stability-chart-scroll');
 if(existingChart){
  if(summaryChanged)updateDetailSummary(summary);
  if(samplesChanged)updateDetailChart(existingChart,samples,true,detailFollowLatest);
 } else {
  content.innerHTML=`<div id="detail-realtime-view"><div id="detail-stability-result" class="stability-result tone-${toneName}"><span class="stability-result-dot"></span><strong id="detail-stability-label">${esc(label)}</strong></div><div class="detail-grid">${detailMetric('detail-p50','P50 典型延迟',detailMs(summary.p50_ms))}${detailMetric('detail-p90','P90 较高延迟',detailMs(summary.p90_ms))}${detailMetric('detail-success-rate','成功率',detailPercent(summary.success_rate))}${detailMetric('detail-high-latency','高延迟比例',detailPercent(summary.high_latency_rate))}${detailMetric('detail-max-failures','最大连续失败',String(summary.max_consecutive_failures||0)+' 次')}${detailMetric('detail-samples','探测次数',String(summary.samples||0)+' 次')}</div><div class="stability-chart-heading"><span>真实延迟</span><span class="subtle">每根柱子代表一次探测 · 横向滚动查看</span></div><div class="stability-chart-scroll" role="img" aria-label="最近 24 小时真实延迟"><div class="stability-chart-track"><svg class="stability-chart-layer" height="80" preserveAspectRatio="none"></svg></div></div><div class="detail-note">绿色、黄色和红色跟随当前延迟阈值；灰色表示失败。样本从 smart 启动或切换机场后重新积累。</div></div>`;
  const chart=content.querySelector('.stability-chart-scroll');
  paintDetailChart(chart,samples);
  requestAnimationFrame(()=>{chart.scrollLeft=Math.max(0,chart.scrollWidth-chart.clientWidth);});
 }
 detailSummaryKey=summaryKey;
 detailSamplesKey=samplesKey;
}
async function loadNodeDetail() {
 if(!detailNodeId)return;
 const nodeID=detailNodeId, token=++detailRequestToken;
 detailRequestController?.abort();
 const controller=new AbortController();
 detailRequestController=controller;
 try {
  const previous=detailData;
  const cursor=previous?.samples?.length?previous.samples[previous.samples.length-1].at:'';
  const query=cursor?'?since='+encodeURIComponent(cursor):'';
  const response=await fetch('/api/node/'+encodeURIComponent(nodeID)+'/stability'+query,{cache:'no-store',signal:controller.signal});
  if(!response.ok)throw new Error('节点详情读取失败');
  const next=await response.json();
  if(token!==detailRequestToken||detailNodeId!==nodeID)return false;
  if(previous&&cursor){
   const cutoff=Date.now()-24*60*60*1000;
   next.samples=[...previous.samples,...(next.samples||[])].filter(sample=>new Date(sample.at).getTime()>=cutoff);
  }
  detailData=next;
  renderDetail(detailData);
  return true;
 } catch(error) {
  if(error.name==='AbortError'||token!==detailRequestToken||detailNodeId!==nodeID)return false;
  $('node-detail-content').innerHTML=`<div class="detail-empty">${esc(error.message)}</div>`;
  return false;
 } finally {
  if(detailRequestController===controller)detailRequestController=null;
 }
}
function closeDetailStream() {
 if(detailStream){detailStream.close();detailStream=null;}
}
function connectDetailStream() {
 closeDetailStream();
 if(!detailNodeId||!window.EventSource)return;
 const nodeID=detailNodeId;
 detailNodeSequence=0;
 const source=new EventSource('/api/node/'+encodeURIComponent(nodeID)+'/stability/events');
 detailStream=source;
 source.addEventListener('stability',event=>{
  if(detailNodeId!==nodeID)return;
  let update;
  try { update=JSON.parse(event.data); } catch(_) { return; }
  detailUpdateChain=detailUpdateChain.then(async()=>{
   if(detailNodeId!==nodeID)return;
   const sequence=Number(update.node_sequence)||0;
   if(detailNodeSequence&&sequence>detailNodeSequence+1){
    await loadNodeDetail();
   }
   if(detailNodeId!==nodeID)return;
   const previous=detailData||{samples:[]}, samples=[...(previous.samples||[])], sample=update.sample;
   const last=samples[samples.length-1];
   if(sample?.at&&(!last||sample.at>last.at))samples.push(sample);
   const cutoff=Date.now()-24*60*60*1000;
   detailData={summary:update.summary,samples:samples.filter(sample=>new Date(sample.at).getTime()>=cutoff)};
   detailNodeSequence=sequence||detailNodeSequence;
   renderDetail(detailData);
  }).catch(()=>{});
 });
 source.onerror=()=>{
  const active=detailStream===source;
  if(active)detailStream=null;
  source.close();
  if(active&&detailNodeId===nodeID&&!detailTimer){detailTimer=setTimeout(async()=>{detailTimer=0;if(detailNodeId===nodeID){await loadNodeDetail();connectDetailStream();}},3000);}
 };
}
function openNodeDetail(id) {
 if(!id)return;
 closeDetailStream();
 detailNodeId=id;detailData=null;detailSummaryKey='';detailSamplesKey='';detailNodeSequence=0;detailUpdateChain=Promise.resolve();resetDetailView();detailFollowLatest=true;
 const dialog=$('node-detail-dialog');
 if(!dialog.open)dialog.showModal();
 dialog.focus({preventScroll:true});
 $('node-detail-content').innerHTML='<div class="detail-empty">正在读取节点数据…</div>';
 loadNodeDetail().then(connectDetailStream);
}
function closeNodeDetail() {
 closeDetailStream();
 detailRequestController?.abort();detailLongTermController?.abort();detailRequestToken++;
 detailNodeId='';detailData=null;detailSummaryKey='';detailSamplesKey='';detailNodeSequence=0;detailUpdateChain=Promise.resolve();resetDetailView();detailFollowLatest=true;
 const dialog=$('node-detail-dialog');if(dialog.open)dialog.close();
}
