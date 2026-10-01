package app

import (
	"context"
	"io"
	"log"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/demo"
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func newFlowApp(t *testing.T) *App {
	t.Helper()
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.phase, a.current, a.pending, a.pendingKind = "normal", "fixed", "hold", ""
	return a
}

func TestHighLatencyRecoverySuccessStaysNormal(t *testing.T) {
	a := newFlowApp(t)
	n := a.store.Nodes[1]
	n.Tier = domain.TierOrdinary
	a.store.Record(n.Info.ID, domain.Probe{Success: false, Error: "timeout", At: time.Now()})
	n.Plan.Reason = "normal"
	a.observe(context.Background(), observation{id: n.Info.ID, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 800, At: time.Now()}})
	if n.Plan.RecoveryStep != 0 {
		t.Fatalf("high-latency recovery entered fast review: %d", n.Plan.RecoveryStep)
	}
}

func TestLatencyDropStartsRecoveryReviewAndKeepsScoring(t *testing.T) {
	a := newFlowApp(t)
	n := a.store.Nodes[1]
	n.Tier = domain.TierOrdinary
	for _, delay := range []float64{800, 820, 780} {
		a.store.Record(n.Info.ID, domain.Probe{Success: true, DelayMS: delay, At: time.Now()})
	}
	n.Plan.Reason = "normal"
	a.observe(context.Background(), observation{id: n.Info.ID, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 250, At: time.Now()}})
	if n.Plan.RecoveryStep != 1 {
		t.Fatalf("latency drop did not start recovery: %d", n.Plan.RecoveryStep)
	}
	if n.Checks != 4 || len(n.History) != 4 {
		t.Fatalf("probe was not scored and recorded: checks=%d history=%d", n.Checks, len(n.History))
	}
}
