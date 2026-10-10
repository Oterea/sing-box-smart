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

func TestPauseDiscardsQueuedResultsAndPreservesHistory(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	n := a.store.Nodes[0]
	a.store.Record(n.Info.ID, domain.Probe{Success: true, DelayMS: 120, At: time.Now()})
	before := n.Checks
	gen := a.generation
	n.Plan.InFlight = true
	a.pending = n.Info.ID
	a.pendingKind = "node"
	a.pauseDetection()
	a.tick(ctx, time.Now())
	a.observe(ctx, observation{id: n.Info.ID, generation: gen, probe: domain.Probe{Success: false, At: time.Now()}})
	if n.Checks != before || n.Plan.InFlight || a.pending != "" || !a.snapshot().Paused {
		t.Fatal("pause did not preserve/cancel state")
	}
	if err := a.control(ctx, "recheck", ""); err == nil {
		t.Fatal("paused recheck accepted")
	}
	a.resumeDetection()
	if a.paused || a.probeCtx.Err() != nil || n.Plan.NormalDue.After(time.Now()) {
		t.Fatal("resume failed")
	}
}

func TestResumeRunsOneFullRefreshBeforeNormalScheduling(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer a.probeCancel()
	a.phase = "normal"
	a.current = a.store.Nodes[0].Info.ID
	a.selectionMode = domain.SelectionManual
	a.pauseDetection()
	a.resumeDetection()

	if a.phase != "refresh" || !a.refresh.Active() {
		t.Fatalf("resume did not start refresh: phase=%q active=%v", a.phase, a.refresh.Active())
	}
	now := time.Now()
	for i, n := range a.store.Nodes {
		n.Plan.Reason = "resume"
		a.observe(context.Background(), observation{
			id:         n.Info.ID,
			generation: a.generation,
			probe:      domain.Probe{Success: true, DelayMS: float64(100 + i), At: now},
		})
		if i < len(a.store.Nodes)-1 && a.phase != "refresh" {
			t.Fatalf("refresh ended before node %d completed", i)
		}
	}
	if a.phase != "normal" || a.refresh.Active() {
		t.Fatalf("refresh did not finish: phase=%q active=%v", a.phase, a.refresh.Active())
	}
	for _, n := range a.store.Nodes {
		if n.Checks != 1 {
			t.Fatalf("node %q checks=%d, want one refresh check", n.Info.ID, n.Checks)
		}
		if n.Plan.NormalDue.Before(now) {
			t.Fatalf("node %q was not returned to normal scheduling", n.Info.ID)
		}
	}
}

func TestResumeRefreshDoesNotTriggerRecoveryOrDuplicateOnSecondResume(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer a.probeCancel()
	a.phase = "normal"
	a.pauseDetection()
	a.resumeDetection()
	firstStarted := a.refresh.Started()
	a.resumeDetection()
	if !a.refresh.Active() || !a.refresh.Started().Equal(firstStarted) {
		t.Fatal("repeated resume created a duplicate refresh cycle")
	}
	n := a.store.Nodes[1]
	n.Tier = domain.TierOrdinary
	n.Plan.Reason = "resume"
	a.observe(context.Background(), observation{id: n.Info.ID, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 900, At: time.Now()}})
	if n.Plan.RecoveryStep != 0 {
		t.Fatalf("resume refresh entered recovery review: %d", n.Plan.RecoveryStep)
	}
}
