package app

import (
	"context"
	"io"
	"log"
	"os"
	"path/filepath"
	"reflect"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/demo"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/preferences"
	"sing-box-smart/internal/sleep"
	"testing"
	"time"
)

func preferenceApp(t *testing.T) *App {
	t.Helper()
	cfg := config.Default()
	cfg.SettingsPath = filepath.Join(t.TempDir(), "connection.json")
	a, err := New(context.Background(), cfg, demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(a.probeCancel)
	return a
}
func TestDisplaySettingsPreserveDetectionRecoveryAndScores(t *testing.T) {
	a := preferenceApp(t)
	a.phase = "normal"
	n := a.store.Nodes[0]
	a.store.Record(n.Info.ID, domain.Probe{At: time.Now(), Success: true, DelayMS: 100})
	n.Plan.StartRecovery(time.Now(), a.cfg.RecoverySteps)
	beforePlan := n.Plan
	beforeMetrics := n.Metrics
	generation := a.generation
	s := a.cfg.Preferences()
	s.DelayGreen = 100
	s.DelayYellow = 400
	s.LogLevel = "debug"
	if err := a.applySettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(n.Plan, beforePlan) || n.Metrics != beforeMetrics || a.generation != generation {
		t.Fatal("display change disturbed detection")
	}
	saved, err := preferences.Load(a.cfg.PreferencesPath(), config.Default().Preferences())
	if err != nil || saved.DelayYellow != 400 {
		t.Fatalf("not persisted: %+v %v", saved, err)
	}
	if a.events.MinLevel != events.Debug {
		t.Fatal("log level not applied")
	}
	if a.snapshot().Settings.DelayGreen != 100 {
		t.Fatal("settings missing from snapshot")
	}
}
func TestSettingsFailureHasNoRuntimeSideEffects(t *testing.T) {
	a := preferenceApp(t)
	before := a.cfg.Preferences()
	s := before
	s.DelayGreen = 99
	a.pending = "switch"
	if err := a.applySettings(context.Background(), s); err == nil {
		t.Fatal("allowed pending switch")
	}
	a.pending = ""
	if _, err := os.Stat(a.cfg.PreferencesPath()); !os.IsNotExist(err) {
		t.Fatal("saved during switch")
	}
	a.cfg.SettingsPath = filepath.Join(t.TempDir(), "file", "connection.json")
	if err := os.WriteFile(filepath.Dir(a.cfg.SettingsPath), []byte("file"), 0600); err != nil {
		t.Fatal(err)
	}
	if err := a.applySettings(context.Background(), s); err == nil {
		t.Fatal("accepted failed persistence")
	}
	if !reflect.DeepEqual(a.cfg.Preferences(), before) {
		t.Fatal("changed runtime after save failed")
	}
}
func TestDetectionSettingsCancelOldProbesAndKeepPauseHistory(t *testing.T) {
	a := preferenceApp(t)
	a.paused = true
	a.phase = "normal"
	n := a.store.Nodes[0]
	for i := 0; i < 5; i++ {
		a.store.Record(n.Info.ID, domain.Probe{At: time.Now(), Success: true, DelayMS: float64(100 + i)})
	}
	previous := a.generation
	s := a.cfg.Preferences()
	s.Timeout = 8
	s.Current = 6
	s.HistoryLimit = 3
	if err := a.applySettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if !a.paused || n.Checks != 5 || len(n.History) != 3 || a.generation <= previous || a.runner.Timeout != 8*time.Second {
		t.Fatal("incorrect detection update")
	}
}
func TestSleepThresholdEditDoesNotWakeAndDisablingResumes(t *testing.T) {
	a := preferenceApp(t)
	a.cfg.SleepEnabled = true
	a.cfg.GRPCAddress = "127.0.0.1:9090"
	s := a.cfg.Preferences()
	a.sleepController.Reconfigure(sleep.Config{Enabled: true, IdleAfter: a.cfg.SleepIdleAfter, QuietBytes: a.cfg.SleepQuietBytes, WakeWindow: a.cfg.SleepWakeWindow, WakeBytes: a.cfg.SleepWakeBytes}, time.Now().Add(-11*time.Minute))
	a.sleepController.Tick(time.Now(), false)
	a.enterSleep()
	s.WakeBytes = 2 * 1024 * 1024
	if err := a.applySettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if !a.sleeping {
		t.Fatal("threshold edit woke sleeping controller")
	}
	s.SleepEnabled = false
	if err := a.applySettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if a.sleeping || !a.resumeWindow || a.phase != "startup" {
		t.Fatal("disabling sleep did not resume")
	}
	records := a.sleepRecords.Snapshot(time.Now())
	if len(records) != 1 || records[0].Active {
		t.Fatal("sleep record not closed")
	}
}
func TestSettingsViaEventLoop(t *testing.T) {
	a := preferenceApp(t)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	go a.Run(ctx)
	ctx, timeout := context.WithTimeout(ctx, time.Second)
	defer timeout()
	s := a.cfg.Preferences()
	s.DelayYellow = 500
	if err := a.ConfigureSettings(ctx, s); err != nil {
		t.Fatal(err)
	}
	snapshot, err := a.Snapshot(ctx)
	if err != nil || snapshot.Settings.DelayYellow != 500 {
		t.Fatalf("not applied: %v", err)
	}
}
