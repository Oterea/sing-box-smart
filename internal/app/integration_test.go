package app

import (
	"context"
	"io"
	"log"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/demo"
	"testing"
	"time"
)

func TestRunComposesStartupSchedulingPauseAndResume(t *testing.T) {
	cfg := config.Default()
	cfg.Startup = 20 * time.Millisecond
	cfg.Current = 20 * time.Millisecond
	cfg.Candidate = 30 * time.Millisecond
	cfg.Ordinary = 50 * time.Millisecond
	cfg.Recovery = 10 * time.Millisecond
	cfg.Timeout = time.Second
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	a, err := New(ctx, cfg, demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	go a.Run(ctx)

	deadline := time.Now().Add(2 * time.Second)
	var snapshotDone bool
	for time.Now().Before(deadline) {
		s, err := a.Snapshot(ctx)
		if err != nil {
			t.Fatal(err)
		}
		if s.Phase == "normal" || s.Phase == "unavailable" {
			snapshotDone = true
			if len(s.Nodes) != 12 {
				t.Fatalf("startup node count=%d", len(s.Nodes))
			}
			break
		}
		time.Sleep(20 * time.Millisecond)
	}
	if !snapshotDone {
		t.Fatal("startup did not finish")
	}
	if err := a.Control(ctx, "pause", ""); err != nil {
		t.Fatal(err)
	}
	s, err := a.Snapshot(ctx)
	if err != nil || !s.Paused {
		t.Fatalf("pause snapshot=%+v err=%v", s, err)
	}
	if err := a.Control(ctx, "recheck", ""); err == nil {
		t.Fatal("recheck accepted while paused")
	}
	if err := a.Control(ctx, "resume", ""); err != nil {
		t.Fatal(err)
	}
	s, err = a.Snapshot(ctx)
	if err != nil || s.Paused {
		t.Fatalf("resume snapshot=%+v err=%v", s, err)
	}
}
