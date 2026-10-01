// Package recovery detects a low-frequency node's first sign of improvement.
package recovery

import (
	"sing-box-smart/internal/domain"
	"sort"
)

type Kind string

const (
	None          Kind = ""
	FailureToGood Kind = "failure_to_success"
	LatencyDrop   Kind = "latency_drop"
	CandidateNear Kind = "candidate_near"
)

type Signal struct {
	Kind     Kind
	Baseline float64
}

// Detect uses history before current. A large drop needs both a relative and
// an absolute improvement so ordinary jitter does not start a fast review.
func Detect(history []domain.Sample, previous domain.Probe, current domain.Probe, ratio, minDrop float64) Signal {
	if !current.Success {
		return Signal{}
	}
	if previousError(previous) {
		return Signal{Kind: FailureToGood}
	}
	values := make([]float64, 0, 5)
	for i := len(history) - 1; i >= 0 && len(values) < 5; i-- {
		if history[i].Success && history[i].DelayMS != nil {
			values = append(values, *history[i].DelayMS)
		}
	}
	if len(values) < 2 {
		return Signal{}
	}
	sort.Float64s(values)
	baseline := values[len(values)/2]
	if baseline-current.DelayMS >= minDrop && current.DelayMS <= baseline*ratio {
		return Signal{Kind: LatencyDrop, Baseline: baseline}
	}
	return Signal{}
}

func previousError(p domain.Probe) bool {
	return !p.Success && (p.Error != "" || p.At.IsZero() == false)
}
