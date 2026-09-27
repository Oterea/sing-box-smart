// Package score implements the agreed per-observation formula without I/O or history storage.
package score

import (
	"math"
	"sing-box-smart/internal/domain"
)

func Update(m domain.Metrics, p domain.Probe) domain.Metrics {
	success := 0.0
	if p.Success {
		success = 1
		if m.HasLatency {
			m.L = .8*m.L + .2*p.DelayMS
		} else {
			m.L = p.DelayMS
			m.HasLatency = true
		}
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
	return m.L / denominator, true
}
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
