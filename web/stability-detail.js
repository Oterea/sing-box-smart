// Pure data keys for the node stability detail view.
// Summary metrics and raw probe samples have different update lifecycles:
// a new sample must repaint the chart even when the summary is unchanged.
function stabilitySummaryKey(summary) {
 return JSON.stringify([
  summary?.samples,
  summary?.successes,
  summary?.failures,
  summary?.success_rate,
  summary?.p50_ms,
  summary?.p90_ms,
  summary?.high_latency_rate,
  summary?.max_consecutive_failures
 ]);
}

function stabilitySampleKey(sample) {
 return JSON.stringify([
  sample?.at,
  sample?.success,
  sample?.delay_ms,
  sample?.error
 ]);
}

function stabilitySamplesKey(samples) {
 return JSON.stringify((samples || []).map(stabilitySampleKey));
}
