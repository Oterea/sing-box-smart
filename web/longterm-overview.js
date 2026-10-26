// Long-term comparison table. The backend supplies compact summaries for all
// nodes; this module only groups, ranks and renders them.
const overviewElement=id=>document.getElementById(id);
let longTermOverviewData=null, longTermOverviewAirport='', longTermOverviewLoadedAt=0, longTermOverviewController=null, longTermOverviewLoadingAirport='', longTermOverviewThresholdKey='', longTermOverviewRetryAt=0, longTermOverviewError='', longTermOverviewState=null;
let longTermOverviewWindow=14, longTermOverviewSorts=['p50','p90','success','high'], longTermOverviewExpanded={};
const longTermOverviewMetrics=[{key:'p50',label:'P50（ms）/ 排名',direction:'asc'},{key:'p90',label:'P90（ms）/ 排名',direction:'asc'},{key:'success',label:'成功率（%）/ 排名',direction:'desc'},{key:'high',label:'高延迟（%）/ 排名',direction:'asc'},{key:'fail',label:'连续失败 / 排名',direction:'asc'}];
const longTermOverviewGroups=['稳定低延迟','稳定中延迟','稳定高延迟','不稳定','样本不足'];
function overviewSummary(node,days){return (node.windows||[]).find(window=>Number(window.days)===days)?.summary||{};}
function overviewEscape(value){return String(value??'').replace(/[&<>"']/g,c=>({'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'}[c]));}
function overviewValue(summary,key){const value=key==='p50'?Number(summary.p50_ms):key==='p90'?Number(summary.p90_ms):key==='success'?Number(summary.success_rate):key==='high'?Number(summary.high_latency_rate):Number(summary.max_consecutive_failures);return Number.isFinite(value)?value:null;}
function overviewFormat(summary,key){const value=overviewValue(summary,key);if(value==null)return '—';return key==='p50'||key==='p90'?String(Math.round(value)):key==='success'||key==='high'?(value*100).toFixed(1):String(Math.round(value));}
function overviewEligible(summary,key){return Number(summary.samples)>=10&&overviewValue(summary,key)!=null;}
function overviewRanks(nodes,days){
 const summaries=new Map(nodes.map(node=>[node.id,overviewSummary(node,days)]));
 const result=new Map();
 for(const metric of longTermOverviewMetrics){
  const values=nodes.map(node=>({id:node.id,value:overviewValue(summaries.get(node.id),metric.key)})).filter(item=>overviewEligible(summaries.get(item.id),metric.key));
  values.sort((a,b)=>(a.value-b.value)*(metric.direction==='asc'?1:-1)||a.id.localeCompare(b.id));
  let previous=null,rank=0;
  for(let index=0;index<values.length;index++){if(previous===null||values[index].value!==previous){rank=index+1;previous=values[index].value;}if(!result.has(values[index].id))result.set(values[index].id,{});result.get(values[index].id)[metric.key]=rank;}
 }
 return result;
}
function overviewColumnCount(){return longTermOverviewMetrics.length+2;}
function overviewClass(summary,snapshot=longTermOverviewState){return stabilityConclusion(summary,Number(snapshot?.settings?.delay_green_ms??200),Number(snapshot?.settings?.delay_yellow_ms??600));}
function overviewSort(nodes,days){return [...nodes].sort((a,b)=>{const as=overviewSummary(a,days),bs=overviewSummary(b,days);for(const key of longTermOverviewSorts){const metric=longTermOverviewMetrics.find(item=>item.key===key),av=overviewValue(as,key),bv=overviewValue(bs,key);if(av==null&&bv==null)continue;if(av==null)return 1;if(bv==null)return -1;if(av!==bv)return(av-bv)*(metric.direction==='asc'?1:-1);}return a.name.localeCompare(b.name);});}
function overviewTable(nodes,days,snapshot){
 const ranks=overviewRanks(nodes,days),groups=new Map(longTermOverviewGroups.map(group=>[group,[]]));
 for(const node of nodes){const label=overviewClass(overviewSummary(node,days))[0];(groups.get(label)||groups.get('样本不足')).push(node);}
 const configured=Number(snapshot?.settings?.long_term_display_limit??5),limit=Number.isInteger(configured)&&configured>=1&&configured<=1000?configured:5;
 const columns=overviewColumnCount();
 return longTermOverviewGroups.map(group=>{const rows=overviewSort(groups.get(group)||[],days);if(!rows.length)return '';const visible=longTermOverviewExpanded[group]?rows:rows.slice(0,limit),tone=group==='稳定低延迟'?'good':group==='稳定中延迟'?'medium':group==='样本不足'?'none':'bad';const toggle=rows.length>limit?`<tr class="longterm-group-toggle-row"><td colspan="${columns}"><div class="node-list-controls"><button class="button secondary" data-longterm-toggle-group="${overviewEscape(group)}" type="button">${longTermOverviewExpanded[group]?`收起至前 ${limit} 个`:`显示全部（还有 ${rows.length-limit} 个）`}</button></div></td></tr>`:'';return `<tr class="longterm-group-row"><th colspan="${columns}"><span class="longterm-group-dot tone-${tone}"></span>${group}<span class="subtle">${rows.length} 个</span></th></tr>`+visible.map(node=>{const summary=overviewSummary(node,days);return `<tr><td><button class="node-choice longterm-node-choice" title="${overviewEscape(node.name)}" type="button" tabindex="-1" aria-disabled="true">${overviewEscape(node.name)}</button></td>${longTermOverviewMetrics.map(metric=>`<td><span class="longterm-metric"><span class="longterm-value">${overviewEscape(overviewFormat(summary,metric.key))}</span><span class="longterm-rank"> / ${overviewEscape(String(ranks.get(node.id)?.[metric.key]??'—'))}</span></span></td>`).join('')}<td><button class="detail-button" data-detail-node="${overviewEscape(node.id)}" type="button">详情</button></td></tr>`;}).join('')+toggle;}).join('');
}
function renderLongTermOverview(){
 const panel=overviewElement('longterm-overview-panel'),body=overviewElement('longterm-overview-body');if(!panel||!body)return;
 for(const button of panel.querySelectorAll('[data-longterm-window]'))button.classList.toggle('active',Number(button.dataset.longtermWindow)===longTermOverviewWindow);
 for(const button of panel.querySelectorAll('[data-longterm-sort]')){const active=longTermOverviewSorts.includes(button.dataset.longtermSort);button.classList.toggle('active',active);button.setAttribute('aria-pressed',String(active));}
 const columns=overviewColumnCount();
 if(longTermOverviewError){body.innerHTML=`<tr><td colspan="${columns}" class="empty">${overviewEscape(longTermOverviewError)}，稍后自动重试</td></tr>`;return;}
 if(!longTermOverviewData){body.innerHTML=`<tr><td colspan="${columns}" class="empty">正在读取长期表现…</td></tr>`;return;}
 const nodes=longTermOverviewData.nodes||[];body.innerHTML=nodes.length?overviewTable(nodes,longTermOverviewWindow,longTermOverviewState):`<tr><td colspan="${columns}" class="empty">暂无长期表现数据</td></tr>`;
}
async function loadLongTermOverview(airportID,force=false){
 if(!airportID||(!force&&longTermOverviewLoadingAirport===airportID)||(!force&&Date.now()<longTermOverviewRetryAt)||(!force&&airportID===longTermOverviewAirport&&longTermOverviewData&&Date.now()-longTermOverviewLoadedAt<300000))return;
 longTermOverviewController?.abort();const controller=new AbortController();longTermOverviewController=controller;
 longTermOverviewLoadingAirport=airportID;longTermOverviewAirport=airportID;
 try{const response=await fetch('/api/nodes/stability/overview',{cache:'no-store',signal:controller.signal});if(!response.ok)throw new Error('长期表现读取失败');const data=await response.json();if(controller!==longTermOverviewController||airportID!==longTermOverviewState?.airport_id)return;longTermOverviewData=data;longTermOverviewAirport=airportID;longTermOverviewLoadedAt=Date.now();longTermOverviewRetryAt=0;longTermOverviewError='';renderLongTermOverview();}
 catch(error){if(error.name!=='AbortError'&&controller===longTermOverviewController){longTermOverviewData=null;longTermOverviewError='暂时无法读取长期表现';longTermOverviewRetryAt=Date.now()+30000;renderLongTermOverview();}}
 finally{if(longTermOverviewController===controller){longTermOverviewController=null;longTermOverviewLoadingAirport='';}}
}
function syncLongTermOverview(s){if(!s?.airport_id)return;longTermOverviewState=s;const previousThresholds=JSON.parse(longTermOverviewThresholdKey||'[]'),currentThresholds=[s.settings?.delay_green_ms,s.settings?.delay_yellow_ms],thresholdKey=JSON.stringify(currentThresholds),yellowChanged=previousThresholds.length>1&&previousThresholds[1]!==currentThresholds[1];if(thresholdKey!==longTermOverviewThresholdKey){longTermOverviewThresholdKey=thresholdKey;renderLongTermOverview();}if(s.airport_id!==longTermOverviewAirport){longTermOverviewData=null;longTermOverviewLoadedAt=0;longTermOverviewRetryAt=0;longTermOverviewError='';longTermOverviewExpanded={};renderLongTermOverview();}loadLongTermOverview(s.airport_id,yellowChanged);}
function initLongTermOverview(){const panel=overviewElement('longterm-overview-panel');if(!panel)return;panel.addEventListener('click',event=>{const windowButton=event.target.closest('[data-longterm-window]');if(windowButton){longTermOverviewWindow=Number(windowButton.dataset.longtermWindow);renderLongTermOverview();return;}const sortButton=event.target.closest('[data-longterm-sort]');if(sortButton){const key=sortButton.dataset.longtermSort;if(longTermOverviewSorts.includes(key)){if(longTermOverviewSorts.length>1)longTermOverviewSorts=longTermOverviewSorts.filter(item=>item!==key);}else longTermOverviewSorts=[...longTermOverviewSorts,key];renderLongTermOverview();return;}const groupButton=event.target.closest('[data-longterm-toggle-group]');if(groupButton){const group=groupButton.dataset.longtermToggleGroup;longTermOverviewExpanded[group]=!longTermOverviewExpanded[group];renderLongTermOverview();return;}const detail=event.target.closest('[data-detail-node]');if(detail&&!detail.disabled)openNodeDetail(detail.dataset.detailNode);});renderLongTermOverview();}
initLongTermOverview();
