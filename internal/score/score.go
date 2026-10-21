// Package score implements the agreed per-observation formula without I/O or history storage.
package score

import (
	"math"
	"sing-box-smart/internal/domain"
)

const (
	jitterDecay      = 0.9
	jitterUpdate     = 1 - jitterDecay
	minJitterSamples = 10
	maxJitterPenalty = 0.5
)

func Update(m domain.Metrics, p domain.Probe) domain.Metrics {
	success := 0.0
	if p.Success {
		success = 1
		if m.HasLatency {
			deviation := math.Abs(p.DelayMS - m.L)
			m.J = jitterDecay*m.J + jitterUpdate*deviation
			m.L = .8*m.L + .2*p.DelayMS
		} else {
			m.L = p.DelayMS
			m.HasLatency = true
		}
		m.Successes++
	}
	m.S = .95*m.S + success
	m.N = .95*m.N + 1
	m.F = .9*m.F + .1*(1-success)
	return m
}

// Value may be +Inf after extreme failure runs; comparison uses it without a score cap.
func Value(m domain.Metrics) (float64, bool) {
	if !m.HasLatency {
		return 0, false
	}
	p := (m.S + 1) / (m.N + 2)
	denominator := math.Pow(p, 3) * math.Pow(1-m.F, 3)
	if denominator <= 0 {
		return math.Inf(1), true
	}
	return m.L / denominator * JitterPenalty(m), true
}

// JitterPenalty returns the bounded score multiplier for successful latency
// variation. The warm-up period keeps early, sparse samples from influencing
// selection before the node has enough successful observations.
func JitterPenalty(m domain.Metrics) float64 {
	if !m.HasLatency || m.Successes < minJitterSamples {
		return 1
	}
	ratio := m.J / math.Max(m.L, 1)
	if ratio < 0 {
		ratio = 0
	}
	return 1 + math.Min(maxJitterPenalty, maxJitterPenalty*ratio)
}

// JitterMS returns the smoothed absolute deviation of successful probes.
func JitterMS(m domain.Metrics) float64 { return m.J }

func Display(m domain.Metrics) (*float64, bool) {
	v, ok := Value(m)
	if !ok {
		return nil, false
	}
	if math.IsInf(v, 0) || math.IsNaN(v) {
		return nil, true
	}
	return &v, false
}
