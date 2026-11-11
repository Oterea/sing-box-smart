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

func TestResumeUsesFiveSecondObservationWindow(t *testing.T) {
	cfg := config.Default()
	cfg.Resume = 50 * time.Millisecond
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer a.probeCancel()
	a.cfg = cfg
	a.phase = "normal"
	a.current = a.store.Nodes[0].Info.ID
	a.selectionMode = domain.SelectionManual
	for _, n := range a.store.Nodes {
		a.store.Record(n.Info.ID, domain.Probe{Success: true, DelayMS: 300, At: time.Now()})
	}
	a.pauseDetection()
	a.resumeDetection()

	if a.phase != "startup" || a.observeWindow.Duration() != cfg.Resume {
		t.Fatalf("resume did not start five-second-style observation: phase=%q duration=%s", a.phase, a.observeWindow.Duration())
	}
	now := time.Now()
	for i, n := range a.store.Nodes {
		n.Plan.Reason = "startup"
		a.observe(context.Background(), observation{
			id:         n.Info.ID,
			generation: a.generation,
			probe:      domain.Probe{Success: true, DelayMS: float64(100 + i), At: now},
		})
		if i < len(a.store.Nodes)-1 && a.phase != "startup" {
			t.Fatalf("observation ended before node %d completed", i)
		}
	}
	if a.phase != "startup" {
		t.Fatalf("observation ended before its time window: phase=%q", a.phase)
	}
	for _, n := range a.store.Nodes {
		if n.Checks != 2 {
			t.Fatalf("node %q checks=%d, want one new observation after existing history", n.Info.ID, n.Checks)
		}
	}
	a.tick(context.Background(), a.observeWindow.Started().Add(cfg.Resume))
	if a.phase != "normal" {
		t.Fatalf("observation did not finish after duration and coverage: phase=%q", a.phase)
	}
	_ = now
}

func TestResumeDoesNotCreateDuplicateWindow(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer a.probeCancel()
	a.phase = "normal"
	a.pauseDetection()
	a.resumeDetection()
	firstStarted := a.observeWindow.Started()
	a.resumeDetection()
	if a.phase != "startup" || !a.observeWindow.Started().Equal(firstStarted) {
		t.Fatal("repeated resume created a duplicate observation window")
	}
}

func TestManualRecheckRunsConfiguredRoundsAndScoresEveryResult(t *testing.T) {
	cfg := config.Default()
	cfg.ManualCheckCount = 2
	a, err := New(context.Background(), cfg, demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.phase = "normal"
	before := make(map[string]int, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		before[n.Info.ID] = n.Checks
	}
	if err := a.control(context.Background(), "recheck", ""); err != nil {
		t.Fatal(err)
	}
	for round := 0; round < cfg.ManualCheckCount; round++ {
		ids := make([]string, 0, len(a.store.Nodes))
		for id := range a.manualCheckPending {
			ids = append(ids, id)
		}
		for _, id := range ids {
			n := a.store.ByID[id]
			a.observe(context.Background(), observation{id: id, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 120, At: time.Now()}})
			if n.Checks != before[id]+round+1 {
				t.Fatalf("node %q checks=%d after round %d", id, n.Checks, round+1)
			}
		}
	}
	if a.manualCheckActive() || a.manualCheckRound != 0 || a.manualCheckTotal != 0 {
		t.Fatalf("manual check did not finish: active=%v round=%d total=%d", a.manualCheckActive(), a.manualCheckRound, a.manualCheckTotal)
	}
}

func TestManualRecheckTickPreservesManualReason(t *testing.T) {
	cfg := config.Default()
	cfg.ManualCheckCount = 2
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, cfg, demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.phase = "normal"
	if err := a.control(ctx, "recheck", ""); err != nil {
		t.Fatal(err)
	}
	a.tick(ctx, time.Now())
	for _, n := range a.store.Nodes {
		if n.Plan.Reason != "manual" {
			t.Fatalf("node %q reason=%q, want manual", n.Info.ID, n.Plan.Reason)
		}
	}
	a.probeCancel()
}

func TestManualRecheckRejectsUnavailableOrBusyState(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.healthy = false
	if err := a.control(context.Background(), "recheck", ""); err == nil {
		t.Fatal("recheck accepted while API was unhealthy")
	}
	a.healthy = true
	a.pending = "node"
	if err := a.control(context.Background(), "recheck", ""); err == nil {
		t.Fatal("recheck accepted while a switch was pending")
	}
}

func TestManualRecheckSuppressesAutomaticSelectionUntilComplete(t *testing.T) {
	cfg := config.Default()
	a, err := New(context.Background(), cfg, demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.phase = "normal"
	a.selectionMode = domain.SelectionAuto
	a.controlActive = true
	a.selectionInitialized = true
	a.current = a.store.Nodes[0].Info.ID
	for _, n := range a.store.Nodes {
		a.store.Record(n.Info.ID, domain.Probe{Success: true, DelayMS: 500, At: time.Now()})
	}
	if err := a.control(context.Background(), "recheck", ""); err != nil {
		t.Fatal(err)
	}
	id := a.store.Nodes[0].Info.ID
	a.observe(context.Background(), observation{id: id, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 50, At: time.Now()}})
	if a.pending != "" || a.switchBusy {
		t.Fatalf("manual result started automatic selection: pending=%q busy=%v", a.pending, a.switchBusy)
	}
}
