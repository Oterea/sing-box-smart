// Package tiering derives the fast candidate band from the current score
// distribution. It has no clocks, I/O, or switching side effects.
package tiering

import (
	"math"
	"sing-box-smart/internal/domain"
	"sort"
)

type Node struct {
	ID        string
	Score     float64
	HasScore  bool
	Available bool
	Previous  domain.Tier
}

type Result struct {
	Tiers       map[string]domain.Tier
	Boundary    int
	HasBoundary bool
}

type scored struct {
	Node
	Score float64
}

// Classify finds a dominant score gap. Nodes before that gap are candidates;
// without a convincing gap, all currently usable scored nodes remain in the
// candidate band rather than inventing a rank cut such as "top ten".
func Classify(nodes []Node) Result {
	r := Result{Tiers: make(map[string]domain.Tier, len(nodes))}
	for _, n := range nodes {
		r.Tiers[n.ID] = domain.TierOrdinary
	}
	valid := make([]scored, 0, len(nodes))
	for _, n := range nodes {
		if n.Available && n.HasScore && !math.IsNaN(n.Score) && !math.IsInf(n.Score, 0) {
			valid = append(valid, scored{Node: n, Score: n.Score})
		}
	}
	sort.SliceStable(valid, func(i, j int) bool {
		if valid[i].Score == valid[j].Score {
			return valid[i].ID < valid[j].ID
		}
		return valid[i].Score < valid[j].Score
	})
	if len(valid) == 0 {
		return r
	}
	frontier := len(valid)
	if len(valid) > 1 {
		gaps := make([]float64, len(valid)-1)
		for i := range gaps {
			gaps[i] = (valid[i+1].Score - valid[i].Score) / math.Max(valid[i].Score, 1)
		}
		largest, second, typical := 0.0, 0.0, 0.0
		for _, gap := range gaps {
			if gap > largest {
				second, largest = largest, gap
			} else if gap > second {
				second = gap
			}
		}
		// Estimate normal spacing after removing the largest gap. This keeps
		// a three-node set with one obvious split from being rejected because
		// the outlier was included in its own baseline.
		typicalGaps := make([]float64, 0, len(gaps)-1)
		removed := false
		for _, gap := range gaps {
			if !removed && gap == largest {
				removed = true
				continue
			}
			typicalGaps = append(typicalGaps, gap)
		}
		sort.Float64s(typicalGaps)
		if len(typicalGaps) > 0 {
			typical = typicalGaps[len(typicalGaps)/2]
		}
		// A boundary must be materially visible and an outlier compared with
		// the normal spacing. This is a distribution rule, not a node count.
		if largest >= 0.25 && largest >= 2*second && largest >= 2.5*typical {
			for i, gap := range gaps {
				if gap == largest {
					frontier = i + 1
					r.Boundary, r.HasBoundary = frontier, true
					break
				}
			}
		}
	}
	for i, n := range valid {
		if i < frontier || (r.HasBoundary && n.Previous == domain.TierCandidate && n.Score <= valid[frontier].Score*1.10) {
			r.Tiers[n.ID] = domain.TierCandidate
		}
	}
	return r
}
