// Package decision only compares observations. It neither probes nor switches.
package decision

import "math"

type Candidate struct {
	ID                  string
	Score               float64
	HasScore, Available bool
}

func Choose(nodes []Candidate, current string, ratio float64) string {
	var best *Candidate
	var old *Candidate
	for i := range nodes {
		n := &nodes[i]
		if n.ID == current {
			old = n
		}
		if n.ID == current || !n.Available || !n.HasScore || math.IsNaN(n.Score) {
			continue
		}
		if best == nil || n.Score < best.Score || (n.Score == best.Score && n.ID < best.ID) {
			best = n
		}
	}
	if best == nil {
		return ""
	}
	if current == "" || old == nil || !old.HasScore {
		return best.ID
	}
	if old.Score > best.Score*ratio {
		return best.ID
	}
	return ""
}
