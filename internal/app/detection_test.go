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
