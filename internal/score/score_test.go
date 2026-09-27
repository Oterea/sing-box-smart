package score

import (
	"math"
	"sing-box-smart/internal/domain"
	"testing"
)

func TestFailureKeepsLatencyAndWorsensScore(t *testing.T) {
	m := domain.Metrics{}
	m = Update(m, domain.Probe{Success: true, DelayMS: 200})
	before, ok := Value(m)
	if !ok {
		t.Fatal("first success should have a score")
	}
	m = Update(m, domain.Probe{Error: "timeout"})
	after, _ := Value(m)
	if m.L != 200 {
		t.Fatalf("failure changed L: %v", m.L)
	}
	if after <= before {
		t.Fatalf("failure did not worsen score: before=%v after=%v", before, after)
	}
}
func TestNoSuccessHasNoScore(t *testing.T) {
	m := Update(domain.Metrics{}, domain.Probe{Error: "timeout"})
	if _, ok := Value(m); ok {
		t.Fatal("failure without latency must show no score")
	}
	if _, overflow := Display(m); overflow {
		t.Fatal("unknown score is not overflow")
	}
}
func TestExtremeFailureIsInfinite(t *testing.T) {
	m := domain.Metrics{L: 100, HasLatency: true, F: 1}
	v, ok := Value(m)
	if !ok || !math.IsInf(v, 1) {
		t.Fatalf("got %v %v", v, ok)
	}
}
