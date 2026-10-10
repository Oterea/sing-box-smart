package app

import (
	"context"
	"io"
	"log"
	"sing-box-smart/internal/activity"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/demo"
	"sing-box-smart/internal/sleep"
	"testing"
	"time"
)

func TestAutomaticSleepStopsProbesAndUserActivityResumesObservation(t *testing.T) {
	cfg := config.Default()
	cfg.SleepEnabled = true
	cfg.GRPCAddress = "127.0.0.1:9090"
	cfg.SleepIdleAfter = time.Millisecond
	a, err := New(context.Background(), cfg, demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.sleepController = sleep.New(sleep.Config{Enabled: true, IdleAfter: time.Millisecond}, time.Now().Add(-time.Second))
	a.tick(context.Background(), time.Now())
	if !a.sleeping || a.phase != "sleeping" {
		t.Fatalf("automatic sleep was not entered: sleeping=%v phase=%q", a.sleeping, a.phase)
	}
	a.handleActivity(activity.Event{Kind: activity.ConnectionNew, At: time.Now()})
	if a.sleeping || a.phase != "startup" || !a.resumeWindow {
		t.Fatalf("activity did not start resume observation: sleeping=%v phase=%q resume=%v", a.sleeping, a.phase, a.resumeWindow)
	}
}
