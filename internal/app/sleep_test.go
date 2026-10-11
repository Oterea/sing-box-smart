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
	a.sleepController = sleep.New(sleep.Config{Enabled: true, IdleAfter: time.Millisecond, QuietBytes: 64 * 1024, WakeWindow: 5 * time.Second, WakeBytes: 32 * 1024}, time.Now().Add(-time.Second))
	a.tick(context.Background(), time.Now())
	if !a.sleeping || a.phase != "sleeping" {
		t.Fatalf("automatic sleep was not entered: sleeping=%v phase=%q", a.sleeping, a.phase)
	}
	if records := a.sleepRecords.Snapshot(time.Now()); len(records) != 1 || !records[0].Active || records[0].Reason != "idle" {
		t.Fatalf("sleep record was not started: %+v", records)
	}
	a.handleActivity(activity.Event{Kind: activity.ConnectionNew, At: time.Now(), Domain: "example.com", Destination: "203.0.113.10:443", DownlinkDelta: 32 * 1024})
	if a.sleeping || a.phase != "startup" || !a.resumeWindow {
		t.Fatalf("activity did not start resume observation: sleeping=%v phase=%q resume=%v", a.sleeping, a.phase, a.resumeWindow)
	}
	if records := a.sleepRecords.Snapshot(time.Now()); len(records) != 1 || records[0].Active || records[0].EndedAt == nil || records[0].WakeReason != "connection" || records[0].WakeSource != "example.com → 203.0.113.10:443" {
		t.Fatalf("sleep record was not closed: %+v", records)
	}
}
