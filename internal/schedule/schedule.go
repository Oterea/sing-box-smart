// Package schedule owns only per-node due times and recovery review steps.
package schedule

import (
	"sing-box-smart/internal/domain"
	"time"
)

type Plan struct {
	NormalDue, ExtraDue time.Time
	RecoveryStep        int
	InFlight            bool
	Reason              string
}

func (p *Plan) Due(now time.Time, phase string, current bool) string {
	if p.InFlight {
		return ""
	}
	if phase == "startup" {
		return "startup"
	}
	if !current && p.RecoveryStep > 0 {
		if !now.Before(p.ExtraDue) {
			return "recovery"
		}
		return ""
	}
	if !now.Before(p.NormalDue) {
		return "normal"
	}
	return ""
}

// StartRecovery configures the extra due time and suppresses normal probes
// until that time arrives.
func (p *Plan) StartRecovery(now time.Time, steps []time.Duration) bool {
	if len(steps) == 0 || p.RecoveryStep > 0 {
		return false
	}
	p.RecoveryStep = 1
	p.NormalDue = time.Time{}
	p.ExtraDue = now.Add(steps[0])
	return true
}

func (p *Plan) FinishRecovery(now time.Time, success bool, steps []time.Duration, next time.Duration) {
	if p.RecoveryStep == 0 {
		return
	}
	if success && p.RecoveryStep < len(steps) {
		p.RecoveryStep++
		p.ExtraDue = now.Add(steps[p.RecoveryStep-1])
		return
	}
	p.RecoveryStep = 0
	p.ExtraDue = time.Time{}
	p.NormalDue = now.Add(next)
}

func (p *Plan) ClearRecovery() { p.RecoveryStep, p.ExtraDue = 0, time.Time{} }

func DurationFor(t domain.Tier, current string, id string, currentEvery, candidateEvery, ordinaryEvery time.Duration) time.Duration {
	if id == current {
		return currentEvery
	}
	if t == domain.TierCandidate {
		return candidateEvery
	}
	return ordinaryEvery
}
