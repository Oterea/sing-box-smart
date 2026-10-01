package tiering

import (
	"sing-box-smart/internal/domain"
	"testing"
)

func TestDominantGapCreatesDynamicCandidateBand(t *testing.T) {
	r := Classify([]Node{
		{ID: "a", Score: 100, HasScore: true, Available: true},
		{ID: "b", Score: 108, HasScore: true, Available: true},
		{ID: "c", Score: 118, HasScore: true, Available: true},
		{ID: "d", Score: 260, HasScore: true, Available: true},
		{ID: "e", Score: 280, HasScore: true, Available: true},
	})
	if !r.HasBoundary || r.Tiers["c"] != domain.TierCandidate || r.Tiers["d"] != domain.TierOrdinary {
		t.Fatalf("unexpected tiers: %+v", r)
	}
}

func TestNoDominantGapDoesNotInventRankCut(t *testing.T) {
	r := Classify([]Node{{ID: "a", Score: 100, HasScore: true, Available: true}, {ID: "b", Score: 108, HasScore: true, Available: true}, {ID: "c", Score: 116, HasScore: true, Available: true}})
	if r.HasBoundary || r.Tiers["a"] != domain.TierCandidate || r.Tiers["c"] != domain.TierCandidate {
		t.Fatalf("unexpected tiers: %+v", r)
	}
}

func TestSmallSetCanStillHaveAnObviousGap(t *testing.T) {
	r := Classify([]Node{{ID: "a", Score: 100, HasScore: true, Available: true}, {ID: "b", Score: 110, HasScore: true, Available: true}, {ID: "c", Score: 140, HasScore: true, Available: true}})
	if !r.HasBoundary || r.Tiers["b"] != domain.TierCandidate || r.Tiers["c"] != domain.TierOrdinary {
		t.Fatalf("unexpected tiers: %+v", r)
	}
}

func TestPreviousCandidateHasExitBuffer(t *testing.T) {
	r := Classify([]Node{{ID: "a", Score: 100, HasScore: true, Available: true}, {ID: "b", Score: 110, HasScore: true, Available: true, Previous: domain.TierCandidate}, {ID: "c", Score: 270, HasScore: true, Available: true}})
	if r.Tiers["b"] != domain.TierCandidate {
		t.Fatalf("candidate left too easily: %+v", r)
	}
}

func TestSmoothLargePoolIsBounded(t *testing.T) {
	nodes := make([]Node, 0, 70)
	for i := 0; i < 70; i++ {
		nodes = append(nodes, Node{ID: string(rune('a' + i)), Score: float64(100 + i*3), HasScore: true, Available: true})
	}
	r := Classify(nodes)
	count := 0
	for _, tier := range r.Tiers {
		if tier == domain.TierCandidate {
			count++
		}
	}
	if count != 17 || !r.HasCutoff {
		t.Fatalf("candidate count=%d result=%+v", count, r)
	}
}
