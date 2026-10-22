// Long-term comparison table. The backend supplies compact summaries for all
// nodes; this module only groups, ranks and renders them.
let longTermOverviewData=null, longTermOverviewAirport='', longTermOverviewLoadedAt=0, longTermOverviewController=null, longTermOverviewLoadingAirport='', longTermOverviewThresholdKey='';
let longTermOverviewWindow=14, longTermOverviewSorts=['p50','p90','success','high'];
const longTermOverviewMetrics=[{key:'p50',label:'P50 / 排名',direction:'asc'},{key:'p90',label:'P90 / 排名',direction:'asc'},{key:'success',label:'成功率 / 排名',direction:'desc'},{key:'high',label:'高延迟 / 排名',direction:'asc'}];
const longTermOverviewGroups=['稳定低延迟','稳定中延迟','稳定高延迟','不稳定','样本不足'];
function overviewSummary(node,days){return (node.windows||[]).find(window=>Number(window.days)===days)?.summary||{};}
function overviewValue(summary,key){const value=key==='p50'?Number(summary.p50_ms):key==='p90'?Number(summary.p90_ms):key==='success'?Number(summary.success_rate):Number(summary.high_latency_rate);return Number.isFinite(value)?value:null;}
function overviewFormat(summary,key){const value=overviewValue(summary,key);if(value==null)return '—';return key==='p50'||key==='p90'?Math.round(value)+' ms':(value*100).toFixed(1)+'%';}
function overviewEligible(summary,key){return Number(summary.samples)>=10&&overviewValue(summary,key)!=null;}
function overviewRanks(nodes,days){
 const result=new Map();
 for(const metric of longTermOverviewMetrics){
  const values=nodes.map(node=>({id:node.id,value:overviewValue(overviewSummary(node,days),metric.key)})).filter(item=>{const summary=overviewSummary(nodes.find(node=>node.id===item.id),days);return overviewEligible(summary,metric.key);});
  values.sort((a,b)=>(a.value-b.value)*(metric.direction==='asc'?1:-1)||a.id.localeCompare(b.id));
  let previous=null,rank=0;
  for(let index=0;index<values.length;index++){if(previous===null||values[index].value!==previous){rank=index+1;previous=values[index].value;}if(!result.has(values[index].id))result.set(values[index].id,{});result.get(values[index].id)[metric.key]=rank;}
 }
 return result;
}
function overviewClass(summary){return stabilityConclusion(summary,Number(state?.settings?.delay_green_ms??200),Number(state?.settings?.delay_yellow_ms??600));}
function overviewSort(nodes,days){return [...nodes].sort((a,b)=>{const as=overviewSummary(a,days),bs=overviewSummary(b,days);for(const key of longTermOverviewSorts){const metric=longTermOverviewMetrics.find(item=>item.key===key),av=overviewValue(as,key),bv=overviewValue(bs,key);if(av==null&&bv==null)continue;if(av==null)return 1;if(bv==null)return -1;if(av!==bv)return(av-bv)*(metric.direction==='asc'?1:-1);}return a.name.localeCompare(b.name);});}
function overviewTable(nodes,days){
 const ranks=overviewRanks(nodes,days),groups=new Map(longTermOverviewGroups.map(group=>[group,[]]));
 for(const node of nodes){const label=overviewClass(overviewSummary(node,days))[0];(groups.get(label)||groups.get('样本不足')).push(node);}
 return longTermOverviewGroups.map(group=>{const rows=overviewSort(groups.get(group)||[],days);if(!rows.length)return '';const tone=group==='稳定低延迟'?'good':group==='稳定中延迟'?'medium':group==='样本不足'?'none':'bad';return `<tr class="longterm-group-row"><th colspan="6"><span class="longterm-group-dot tone-${tone}"></span>${group}<span class="subtle">${rows.length} 个</span></th></tr>`+rows.map(node=>{const summary=overviewSummary(node,days);return `<tr><td><span class="longterm-node-name" title="${esc(node.name)}">${esc(node.name)}</span></td>${longTermOverviewMetrics.map(metric=>`<td><span class="longterm-value">${esc(overviewFormat(summary,metric.key))}</span><span class="longterm-rank"> / ${esc(String(ranks.get(node.id)?.[metric.key]??'—'))}</span></td>`).join('')}<td><button class="detail-button" data-detail-node="${esc(node.id)}" type="button">详情</button></td></tr>`;}).join('');}).join('');
}
function renderLongTermOverview(){
 const panel=$('longterm-overview-panel'),body=$('longterm-overview-body');if(!panel||!body)return;
 for(const button of panel.querySelectorAll('[data-longterm-window]'))button.classList.toggle('active',Number(button.dataset.longtermWindow)===longTermOverviewWindow);
 for(const button of panel.querySelectorAll('[data-longterm-sort]')){const active=longTermOverviewSorts.includes(button.dataset.longtermSort);button.classList.toggle('active',active);button.setAttribute('aria-pressed',String(active));}
 if(!longTermOverviewData){body.innerHTML='<tr><td colspan="6" class="empty">正在读取长期表现…</td></tr>';return;}
 const nodes=longTermOverviewData.nodes||[];body.innerHTML=nodes.length?overviewTable(nodes,longTermOverviewWindow):'<tr><td colspan="6" class="empty">暂无长期表现数据</td></tr>';
}
async function loadLongTermOverview(airportID,force=false){
 if(!airportID||(!force&&longTermOverviewLoadingAirport===airportID)||(!force&&airportID===longTermOverviewAirport&&longTermOverviewData&&Date.now()-longTermOverviewLoadedAt<300000))return;
 longTermOverviewController?.abort();const controller=new AbortController();longTermOverviewController=controller;
 longTermOverviewLoadingAirport=airportID;
 try{const response=await fetch('/api/nodes/stability/overview',{cache:'no-store',signal:controller.signal});if(!response.ok)throw new Error('长期表现读取失败');const data=await response.json();if(controller!==longTermOverviewController||airportID!==state?.airport_id)return;longTermOverviewData=data;longTermOverviewAirport=airportID;longTermOverviewLoadedAt=Date.now();renderLongTermOverview();}
 catch(error){if(error.name!=='AbortError'&&controller===longTermOverviewController){longTermOverviewData=null;renderLongTermOverview();}}
 finally{if(longTermOverviewController===controller){longTermOverviewController=null;longTermOverviewLoadingAirport='';}}
}
function syncLongTermOverview(s){if(!s?.airport_id)return;const thresholdKey=JSON.stringify([s.settings?.delay_green_ms,s.settings?.delay_yellow_ms]);if(thresholdKey!==longTermOverviewThresholdKey){longTermOverviewThresholdKey=thresholdKey;renderLongTermOverview();}if(s.airport_id!==longTermOverviewAirport){longTermOverviewData=null;longTermOverviewLoadedAt=0;renderLongTermOverview();}loadLongTermOverview(s.airport_id);}
function initLongTermOverview(){const panel=$('longterm-overview-panel');if(!panel)return;panel.addEventListener('click',event=>{const windowButton=event.target.closest('[data-longterm-window]');if(windowButton){longTermOverviewWindow=Number(windowButton.dataset.longtermWindow);renderLongTermOverview();return;}const sortButton=event.target.closest('[data-longterm-sort]');if(sortButton){const key=sortButton.dataset.longtermSort;if(longTermOverviewSorts.includes(key)){if(longTermOverviewSorts.length>1)longTermOverviewSorts=longTermOverviewSorts.filter(item=>item!==key);}else longTermOverviewSorts=[...longTermOverviewSorts,key];renderLongTermOverview();return;}const detail=event.target.closest('[data-detail-node]');if(detail&&!detail.disabled)openNodeDetail(detail.dataset.detailNode);});renderLongTermOverview();}
initLongTermOverview();
