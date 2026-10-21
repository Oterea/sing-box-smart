// Package stability keeps a bounded, in-memory window of raw probe results.
// It is independent from the switch score: the score is optimized for
// decisions, while this window is optimized for explaining recent behavior.
package stability

import (
	"math"
	"sing-box-smart/internal/domain"
	"sort"
	"time"
)

const Window = 24 * time.Hour

type Tracker struct {
	samples       []domain.StabilitySample
	version       uint64
	cachedVersion uint64
	cachedYellow  float64
	cachedSummary domain.StabilitySummary
	cacheValid    bool
}

func (t *Tracker) Record(p domain.Probe) {
	at := p.At
	if at.IsZero() {
		at = time.Now()
	}
	var delay *float64
	if p.Success {
		value := p.DelayMS
		delay = &value
	}
	t.samples = append(t.samples, domain.StabilitySample{At: at, Success: p.Success, DelayMS: delay, Error: p.Error})
	t.version++
	t.prune(at)
}

func (t *Tracker) prune(now time.Time) {
	cutoff := now.Add(-Window)
	first := 0
	for first < len(t.samples) && t.samples[first].At.Before(cutoff) {
		first++
	}
	if first > 0 {
		t.samples = append([]domain.StabilitySample(nil), t.samples[first:]...)
		t.version++
	}
}

func (t *Tracker) Snapshot(now time.Time) domain.NodeStability {
	return t.SnapshotWithThreshold(now, 600)
}

func (t *Tracker) SnapshotWithThreshold(now time.Time, yellow float64) domain.NodeStability {
	return t.SnapshotWithThresholdSince(now, yellow, time.Time{})
}

func (t *Tracker) SnapshotWithThresholdSince(now time.Time, yellow float64, since time.Time) domain.NodeStability {
	t.prune(now)
	copySamples := append([]domain.StabilitySample(nil), t.samples...)
	summary := summarize(copySamples)
	addHighLatency(&summary, copySamples, yellow)
	if since.IsZero() {
		return domain.NodeStability{Summary: summary, Samples: copySamples}
	}
	newSamples := make([]domain.StabilitySample, 0)
	for _, sample := range copySamples {
		if sample.At.After(since) {
			newSamples = append(newSamples, sample)
		}
	}
	return domain.NodeStability{Summary: summary, Samples: newSamples}
}

func (t *Tracker) Summary(now time.Time, green, yellow float64) domain.StabilitySummary {
	t.prune(now)
	if t.cacheValid && t.cachedVersion == t.version && t.cachedYellow == yellow {
		return t.cachedSummary
	}
	summary := summarize(t.samples)
	addHighLatency(&summary, t.samples, yellow)
	t.cachedVersion = t.version
	t.cachedYellow = yellow
	t.cachedSummary = summary
	t.cacheValid = true
	_ = green
	return summary
}

func summarize(samples []domain.StabilitySample) domain.StabilitySummary {
	summary := domain.StabilitySummary{WindowSeconds: Window.Seconds(), Samples: len(samples)}
	if len(samples) > 0 {
		summary.StartedAt = samples[0].At
	}
	latencies := make([]float64, 0, len(samples))
	consecutive := 0
	for _, sample := range samples {
		if !sample.Success {
			summary.Failures++
			consecutive++
			if consecutive > summary.MaxConsecutiveFails {
				summary.MaxConsecutiveFails = consecutive
			}
			continue
		}
		consecutive = 0
		summary.Successes++
		if sample.DelayMS != nil {
			latencies = append(latencies, *sample.DelayMS)
		}
	}
	if summary.Samples > 0 {
		summary.SuccessRate = float64(summary.Successes) / float64(summary.Samples)
	}
	if len(latencies) > 0 {
		sort.Float64s(latencies)
		p50 := percentile(latencies, .50)
		p90 := percentile(latencies, .90)
		summary.P50MS = &p50
		summary.P90MS = &p90
	}
	return summary
}

func addHighLatency(summary *domain.StabilitySummary, samples []domain.StabilitySample, yellow float64) {
	for _, sample := range samples {
		if sample.Success && sample.DelayMS != nil && *sample.DelayMS > yellow {
			summary.HighLatencyCount++
		}
	}
	if summary.Samples > 0 {
		summary.HighLatencyRate = float64(summary.HighLatencyCount) / float64(summary.Samples)
	}
}

func percentile(values []float64, q float64) float64 {
	if len(values) == 1 {
		return values[0]
	}
	index := q * float64(len(values)-1)
	low := int(math.Floor(index))
	high := int(math.Ceil(index))
	if low == high {
		return values[low]
	}
	return values[low] + (values[high]-values[low])*(index-float64(low))
}
