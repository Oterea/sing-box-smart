// Package schedule decides when each node is due. One owner calls these methods.
package schedule

import "time"

type Plan struct {
	NormalDue, ExtraDue time.Time
	Remaining           int
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
	if !now.Before(p.NormalDue) {
		return "normal"
	}
	if !current && p.Remaining > 0 && !now.Before(p.ExtraDue) {
		return "recovery"
	}
	return ""
}
func (p *Plan) Complete(now time.Time, phase string, current, hadPrevious, previousOK, ok bool, currentEvery, candidateEvery, recoveryEvery time.Duration, budget int) {
	p.InFlight = false
	if phase == "startup" {
		p.NormalDue = now
		return
	}
	if phase == "unavailable" {
		p.Remaining = 0
		p.NormalDue = now.Add(recoveryEvery)
		return
	}
	if current {
		p.Remaining = 0
		p.NormalDue = now.Add(currentEvery)
		return
	}
	if p.Reason == "normal" {
		p.NormalDue = now.Add(candidateEvery)
	}
	if !ok {
		p.Remaining = 0
		return
	}
	if p.Reason == "recovery" && p.Remaining > 0 {
		p.Remaining--
	}
	if hadPrevious && !previousOK {
		p.Remaining = budget
	}
	if p.Remaining > 0 {
		p.ExtraDue = now.Add(recoveryEvery)
	}
}
