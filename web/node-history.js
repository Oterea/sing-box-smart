function historyFrames(previous, next) {
 const oldList=previous??[], newList=next??[];
 if(!oldList.length||!newList.length)return [newList];
 let overlap=0;
 const limit=Math.min(oldList.length,newList.length);
 for(let size=limit;size>0;size--){
  const matches=oldList.slice(-size).every((sample,index)=>sampleKey(sample)===sampleKey(newList[index]));
  if(matches){overlap=size;break;}
 }
 if(!overlap)return [newList];
 const frames=[];
 let frame=oldList.slice();
 for(let index=overlap;index<newList.length;index++){
  frame=[...frame,newList[index]].slice(-20);
  frames.push(frame);
 }
 return frames.length?frames:[newList];
}
function playHistoryFrame(row,best) {
 const frame=row._historyQueue?.shift();
 if(!frame){row._historyPlaying=false;return;}
 row._paintedHistory=frame;
 row.cells[4].innerHTML=history(frame,'score',best,true);
 row.cells[5].innerHTML=history(frame,'delay',best,true);
 row._historyPlaying=true;
 setTimeout(()=>playHistoryFrame(row,best),300);
}
function queueHistory(row,samples,best) {
 const frames=historyFrames(row._paintedHistory,samples);
 row._historyQueue=frames;
 if(!row._historyPlaying)playHistoryFrame(row,best);
}
function history(samples, kind, best, motion=false) {
 const tail=(samples ?? []).slice(-20);
 const ceiling=kind==='delay'?1000:2000;
 // SVG geometry attributes work with the server's strict CSP (no inline styles).
 const bars=tail.map((s,index)=>{
 const value=kind==='score'?s.score:s.delay_ms;
 const x=6+index*15;
  const newest=motion&&index===tail.length-1;
  const title=time(s.at)+' · '+(kind==='score'?'分数 '+scoreNum(value):s.success?'实际延迟 '+num(value)+' ms':'失败：'+s.error)+(kind==='score'&&!s.success?' · 本次探测失败':'');
  let shape;
  if(!s.success){
   // A failed probe has no delay value. Use the same quiet solid marker for
   // both charts instead of inventing a latency or drawing a dashed outline.
   const height=kind==='score'&&Number.isFinite(value)?Math.min(36,Math.max(1,value/ceiling*36)):36;
   shape=`<rect class="chart-failure${newest?' chart-newest':''}" x="${x}" y="${36-height}" width="4" height="${height}"/>`;
  } else if(!Number.isFinite(value)){
   shape=`<path class="chart-missing" d="M${x} 36h5"/>`;
  } else {
   const height=Math.min(36,Math.max(1,value/ceiling*36));
   const color=kind==='score'?tone(value,best):delayTone(value);
   shape=`<rect class="chart-bar bar-${color}${newest?' chart-newest':''}" x="${x}" y="${36-height}" width="4" height="${height}"/>`;
  }
  return `<g><title>${esc(title)}</title>${shape}</g>`;
 }).join('');
 return `<svg class="history-chart${motion?' history-shift':''}" viewBox="0 0 320 36" preserveAspectRatio="none" role="img" aria-label="${kind==='score'?'分数':'延迟'}历史，最近${tail.length}次，旧到新，纵轴上限${Math.round(ceiling)}">${bars}${tail.length?'':'<text x="4" y="23" fill="#95a2ad" font-size="12">暂无记录</text>'}</svg>`;
}
