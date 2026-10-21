package stability

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestTrackerKeepsOnlyTheLastDayAndComputesPercentiles(t *testing.T) {
	now := time.Unix(100000, 0)
	var tracker Tracker
	tracker.Record(domain.Probe{Success: true, DelayMS: 100, At: now.Add(-25 * time.Hour)})
	tracker.Record(domain.Probe{Success: true, DelayMS: 200, At: now.Add(-2 * time.Hour)})
	tracker.Record(domain.Probe{Success: false, Error: "timeout", At: now.Add(-time.Hour)})
	tracker.Record(domain.Probe{Success: true, DelayMS: 400, At: now})
	result := tracker.Snapshot(now)
	if result.Summary.Samples != 3 || result.Summary.Successes != 2 || result.Summary.Failures != 1 {
		t.Fatalf("unexpected summary: %+v", result.Summary)
	}
	if *result.Summary.P50MS != 300 || *result.Summary.P90MS != 380 {
		t.Fatalf("unexpected percentiles: p50=%v p90=%v", *result.Summary.P50MS, *result.Summary.P90MS)
	}
	if result.Summary.MaxConsecutiveFails != 1 {
		t.Fatalf("max consecutive failures=%d", result.Summary.MaxConsecutiveFails)
	}
}

func TestTrackerCountsHighLatencyUsingCurrentThreshold(t *testing.T) {
	now := time.Now()
	var tracker Tracker
	tracker.Record(domain.Probe{Success: true, DelayMS: 700, At: now})
	tracker.Record(domain.Probe{Success: false, At: now.Add(time.Second)})
	summary := tracker.Summary(now.Add(time.Second), 200, 600)
	if summary.HighLatencyCount != 1 || summary.HighLatencyRate != .5 {
		t.Fatalf("unexpected high-latency summary: %+v", summary)
	}
}
