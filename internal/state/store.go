// Package state stores bounded in-memory observations. The app loop is its sole writer.
package state

import (
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/schedule"
	"sing-box-smart/internal/score"
	"time"
)

type Node struct {
	Info    domain.Node
	Metrics domain.Metrics
	Last    domain.Probe
	Checks  int
	History []domain.Sample
	Plan    schedule.Plan
}
type Store struct {
	Nodes []*Node
	ByID  map[string]*Node
	Limit int
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
		frequency := "30 秒"
		if n.Info.ID == current {
			frequency = "2 秒"
		} else if n.Plan.Remaining > 0 {
			frequency = "3 秒复查"
		}
		if phase == "startup" {
			frequency = "连续检查"
		}
		if phase == "unavailable" {
			frequency = "3 秒重试"
		}
		x := domain.NodeView{ID: n.Info.ID, Name: n.Info.Name, Score: v, ScoreOverflow: overflow, Checks: n.Checks, LastSuccess: n.Last.Success, Current: n.Info.ID == current, Probing: n.Plan.InFlight, Frequency: frequency, RecoveryRemaining: n.Plan.Remaining, Samples: append([]domain.Sample{}, n.History...)}
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
func (s *Store) Reschedule(now time.Time, current string, currentEvery, candidateEvery time.Duration) {
	for _, n := range s.Nodes {
		d := candidateEvery
		if n.Info.ID == current {
			d = currentEvery
			n.Plan.Remaining = 0
		}
		n.Plan.NormalDue = now.Add(d)
	}
}
