// Package state stores bounded in-memory observations. The app loop is its sole writer.
package state

import (
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/schedule"
	"sing-box-smart/internal/score"
	"sing-box-smart/internal/tiering"
	"time"
)

type Node struct {
	Info    domain.Node
	Metrics domain.Metrics
	Last    domain.Probe
	Checks  int
	History []domain.Sample
	Plan    schedule.Plan
	Tier    domain.Tier
}
type Store struct {
	Nodes              []*Node
	ByID               map[string]*Node
	Limit              int
	candidateCutoff    float64
	hasCandidateCutoff bool
}

func New(nodes []domain.Node, limit int) *Store {
	s := &Store{ByID: map[string]*Node{}, Limit: limit}
	for _, info := range nodes {
		n := &Node{Info: info}
		s.Nodes = append(s.Nodes, n)
		s.ByID[info.ID] = n
	}
	return s
}
func (s *Store) Record(id string, p domain.Probe) {
	n := s.ByID[id]
	n.Metrics = score.Update(n.Metrics, p)
	n.Last = p
	n.Checks++
	v, _ := score.Display(n.Metrics)
	sample := domain.Sample{At: p.At, Success: p.Success, Score: v, Error: p.Error}
	if p.Success {
		d := p.DelayMS
		sample.DelayMS = &d
	}
	n.History = append(n.History, sample)
	if len(n.History) > s.Limit {
		n.History = n.History[len(n.History)-s.Limit:]
	}
}
func (s *Store) Views(current, phase string) []domain.NodeView {
	out := make([]domain.NodeView, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		v, overflow := score.Display(n.Metrics)
		frequency := "5 分钟"
		if n.Info.ID == current {
			frequency = "3 秒"
		} else if n.Plan.RecoveryStep > 0 {
			frequency = "恢复复查"
		} else if n.Tier == domain.TierCandidate {
			frequency = "30 秒"
		}
		if phase == "startup" {
			frequency = "连续检查"
		}
		if phase == "unavailable" {
			frequency = "3 秒重试"
		}
		x := domain.NodeView{ID: n.Info.ID, Name: n.Info.Name, Score: v, ScoreOverflow: overflow, Checks: n.Checks, LastSuccess: n.Last.Success, Current: n.Info.ID == current, Probing: n.Plan.InFlight, Frequency: frequency, RecoveryRemaining: n.Plan.RecoveryStep, Tier: n.Tier, Samples: append([]domain.Sample{}, n.History...)}
		if n.Checks > 0 {
			t := n.Last.At
			x.LastAt = &t
			if n.Last.Success {
				d := n.Last.DelayMS
				x.DelayMS = &d
			}
		}
		out = append(out, x)
	}
	return out
}
func (s *Store) AllChecked() bool {
	for _, n := range s.Nodes {
		if n.Checks == 0 {
			return false
		}
	}
	return true
}
func (s *Store) AnyAvailable() bool {
	for _, n := range s.Nodes {
		if n.Checks > 0 && n.Last.Success {
			return true
		}
	}
	return false
}
func (n *Node) RecoveryRemaining() int { return n.Plan.RecoveryStep }

func (s *Store) Reclassify(current string) []string {
	inputs := make([]tiering.Node, 0, len(s.Nodes))
	for _, n := range s.Nodes {
		v, ok := score.Value(n.Metrics)
		inputs = append(inputs, tiering.Node{ID: n.Info.ID, Score: v, HasScore: ok, Available: n.Checks > 0 && n.Last.Success, Previous: n.Tier})
	}
	result := tiering.Classify(inputs)
	s.candidateCutoff, s.hasCandidateCutoff = result.Cutoff, result.HasCutoff
	changed := make([]string, 0)
	for _, n := range s.Nodes {
		t := result.Tiers[n.Info.ID]
		if n.Info.ID == current {
			t = domain.TierCurrent
		}
		if t == "" {
			t = domain.TierOrdinary
		}
		if n.Tier != t {
			changed = append(changed, n.Info.ID)
			n.Tier = t
		}
	}
	return changed
}

// NearCandidate reports whether an ordinary node is close enough to the
// current candidate boundary to deserve a short recovery confirmation.
func (s *Store) NearCandidate(id string, margin float64) bool {
	if !s.hasCandidateCutoff || margin <= 0 {
		return false
	}
	n := s.ByID[id]
	if n == nil || n.Tier != domain.TierOrdinary {
		return false
	}
	v, ok := score.Value(n.Metrics)
	return ok && v <= s.candidateCutoff*(1+margin)
}

func (s *Store) Reschedule(now time.Time, current string, currentEvery, candidateEvery, ordinaryEvery time.Duration) {
	counts := map[domain.Tier]int{}
	for _, n := range s.Nodes {
		if n.Info.ID != current {
			counts[n.Tier]++
		}
	}
	seen := map[domain.Tier]int{}
	for _, n := range s.Nodes {
		d := schedule.DurationFor(n.Tier, current, n.Info.ID, currentEvery, candidateEvery, ordinaryEvery)
		n.Plan.ClearRecovery()
		n.Plan.Reason = ""
		n.Plan.NormalDue = now.Add(d)
		if n.Info.ID != current && n.Tier != domain.TierCurrent {
			idx := seen[n.Tier]
			seen[n.Tier]++
			if counts[n.Tier] > 0 {
				n.Plan.NormalDue = now.Add(time.Duration(idx) * d / time.Duration(counts[n.Tier]))
			}
		}
	}
}

func (s *Store) RescheduleNode(now time.Time, n *Node, current string, currentEvery, candidateEvery, ordinaryEvery time.Duration) {
	n.Plan.Reason = ""
	n.Plan.NormalDue = now.Add(schedule.DurationFor(n.Tier, current, n.Info.ID, currentEvery, candidateEvery, ordinaryEvery))
}
