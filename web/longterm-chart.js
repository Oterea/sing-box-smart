// Long-term chart renderer. It owns scale, axis, calendar positioning and
// latency tones; the detail view only supplies a day's buckets and thresholds.
function longTermChartTone(value, green, yellow) {
 return value==null?'none':value<=green?'good':value<=yellow?'medium':'bad';
}
function chartCeiling(green, yellow) {
 const target=Math.max(Number(green)||0,Number(yellow)||0)*1.5;
 return Math.max(100,Math.ceil(target/100)*100);
}
function longTermChart(day, thresholds={}) {
 const green=Math.max(0,Number(thresholds.green)||200), yellow=Math.max(green,Number(thresholds.yellow)||600);
 const width=960,height=150,left=34,right=10,top=10,bottom=24,plotW=width-left-right,plotH=height-top-bottom,ceiling=chartCeiling(green,yellow);
 const points=(day.buckets||[]).map(bucket=>{
  // Calendar conversion is owned by the backend. The chart consumes the
  // authoritative minute value and never interprets the timestamp locally.
  const minutes=Number(bucket.minute_of_day);
  return {bucket,x:left+minutes/1440*plotW,p50:Number(bucket.p50_ms),p90:Number(bucket.p90_ms)};
 }).filter(point=>point.bucket.has_data&&Number.isFinite(point.x)&&point.x>=left&&point.x<=width-right);
 const line=metric=>points.slice(1).map((point,index)=>{
  const previous=points[index], value=point[metric], previousValue=previous[metric];
  if(!Number.isFinite(value)||!Number.isFinite(previousValue)||point.x-previous.x>plotW/24*2)return '';
  const y1=top+plotH-Math.min(ceiling,previousValue)/ceiling*plotH, y2=top+plotH-Math.min(ceiling,value)/ceiling*plotH;
  if(metric==='p90')return `<line class="longterm-p90" x1="${previous.x}" y1="${y1}" x2="${point.x}" y2="${y2}"/>`;
  return `<line class="longterm-line longterm-line-${longTermChartTone(value,green,yellow)}" x1="${previous.x}" y1="${y1}" x2="${point.x}" y2="${y2}"/>`;
 }).join('');
 const gridValues=[0,green,yellow,ceiling].filter((value,index,values)=>value>=0&&value<=ceiling&&values.indexOf(value)===index).sort((a,b)=>a-b);
 const grid=gridValues.map(value=>{const y=top+plotH-value/ceiling*plotH;return `<line class="longterm-grid" x1="${left}" y1="${y}" x2="${width-right}" y2="${y}"/><text class="longterm-axis" x="2" y="${y+3}">${Math.round(value)}</text>`;}).join('');
 const axis=[0,2,4,6,8,10,12,14,16,18,20,22,24].map(hour=>{const x=left+hour/24*plotW,anchor=hour===0?'start':hour===24?'end':'middle';return `<text class="longterm-axis" text-anchor="${anchor}" x="${x}" y="${height-6}">${hour}</text>`;}).join('');
 return `<div class="longterm-chart-panel"><svg class="longterm-chart" viewBox="0 0 ${width} ${height}" role="img" aria-label="${esc(day.date)} 延迟趋势">${grid}${axis}${line('p90')}${line('p50')}</svg></div>`;
}
