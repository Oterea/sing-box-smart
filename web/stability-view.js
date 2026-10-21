// Shared stability presentation rules for the real-time and long-term views.
// The thresholds are passed in so changing settings immediately affects both.
function stabilityConclusion(summary, green, yellow) {
 const samples=Number(summary?.samples)||0;
 const p50=Number(summary?.p50_ms), p90=Number(summary?.p90_ms);
 if(samples<10||!Number.isFinite(p50))return ['样本不足','none'];
 const level=p50<=green?'低延迟':p50<=yellow?'中延迟':'高延迟';
 const high=Number(summary?.high_latency_rate)||0;
 const success=Number(summary?.success_rate)||0;
 const spread=Number.isFinite(p90)&&p50>0?p90/p50:0;
 const unstable=success<.98||high>.1||spread>=2;
 return [(unstable?'不稳定':'稳定')+level,unstable?'bad':p50<=green?'good':p50<=yellow?'medium':'bad'];
}
