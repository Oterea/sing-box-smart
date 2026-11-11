// Package background owns the independent state and staggered schedule used
// for airport strategy groups that are not currently controlled.
package background

import (
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/state"
	"time"
)

type Group struct {
	Airport   domain.Airport
	Store     *state.Store
	CurrentID string
	Due       map[string]time.Time
	InFlight  map[string]bool
}

func New(airport domain.Airport, historyLimit int) *Group {
	return &Group{
		Airport:  airport,
		Store:    state.New(airport.Nodes, historyLimit),
		Due:      map[string]time.Time{},
		InFlight: map[string]bool{},
	}
}

// UpdateAirport refreshes selector membership while retaining observations for
// nodes that still exist. New nodes start clean and removed nodes disappear
// from both the store and the next schedule.
func (g *Group) UpdateAirport(airport domain.Airport, historyLimit int) {
	old := g.Store
	next := state.New(airport.Nodes, historyLimit)
	for _, node := range next.Nodes {
		if previous := old.ByID[node.Info.ID]; previous != nil {
			previous.Info = node.Info
			previous.Plan.InFlight = false
			if len(previous.History) > historyLimit {
				previous.History = previous.History[len(previous.History)-historyLimit:]
			}
			node = previous
		}
		next.ByID[node.Info.ID] = node
	}
	next.Nodes = make([]*state.Node, 0, len(airport.Nodes))
	for _, info := range airport.Nodes {
		next.Nodes = append(next.Nodes, next.ByID[info.ID])
	}
	g.Store = next
	g.Airport = airport
	if g.CurrentID != "" && g.Store.ByID[g.CurrentID] == nil {
		g.CurrentID = ""
	}
	g.Due = map[string]time.Time{}
	g.InFlight = map[string]bool{}
}

// Schedule spreads nodes across the interval instead of probing a whole
// airport selector in one burst.
func (g *Group) Schedule(now time.Time, interval time.Duration) {
	if interval <= 0 {
		interval = 30 * time.Minute
	}
	count := len(g.Store.Nodes)
	for index, node := range g.Store.Nodes {
		g.Due[node.Info.ID] = now.Add(interval * time.Duration(index) / time.Duration(max(1, count)))
		g.InFlight[node.Info.ID] = false
	}
}

func (g *Group) DueNodes(now time.Time) []*state.Node {
	out := make([]*state.Node, 0)
	for _, node := range g.Store.Nodes {
		if g.InFlight[node.Info.ID] || now.Before(g.Due[node.Info.ID]) {
			continue
		}
		g.InFlight[node.Info.ID] = true
		out = append(out, node)
	}
	return out
}

func (g *Group) Complete(id string, now time.Time, interval time.Duration) {
	g.InFlight[id] = false
	g.Due[id] = now.Add(interval)
}

func (g *Group) Reclassify() { g.Store.Reclassify(g.CurrentID) }

func max(a, b int) int {
	if a > b {
		return a
	}
	return b
}
