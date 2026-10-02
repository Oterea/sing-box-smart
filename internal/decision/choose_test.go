package decision

import (
	"math"
	"testing"
)

func TestChooseBoundaries(t *testing.T) {
	nodes := []Candidate{{ID: "current", Score: 140, HasScore: true, Available: true}, {ID: "best", Score: 100, HasScore: true, Available: true}, {ID: "unknown", Available: true}, {ID: "bad", Score: math.NaN(), HasScore: true, Available: true}}
	if got := Choose(nil, "", 1.4); got != "" {
		t.Fatalf("empty chose %q", got)
	}
	if got := Choose(nodes, "", 1.4); got != "best" {
		t.Fatalf("no current chose %q", got)
	}
	if got := Choose(nodes, "current", 1.4); got != "" {
		t.Fatalf("threshold switch chose %q", got)
	}
	nodes[0].Score = 140.1
	if got := Choose(nodes, "current", 1.4); got != "best" {
		t.Fatalf("below threshold switched to %q", got)
	}
}

func TestChooseIgnoresUnavailableAndUsesStableTieBreak(t *testing.T) {
	nodes := []Candidate{{ID: "z", Score: 80, HasScore: true, Available: true}, {ID: "a", Score: 80, HasScore: true, Available: true}, {ID: "offline", Score: 1, HasScore: true}, {ID: "no-score", Available: true}}
	if got := Choose(nodes, "current", 1.4); got != "a" {
		t.Fatalf("tie break chose %q", got)
	}
}
