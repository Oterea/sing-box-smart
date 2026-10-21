package app

import (
	"context"
	"fmt"
	"reflect"
	"sing-box-smart/internal/activity"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/preferences"
	"sing-box-smart/internal/sleep"
	"time"
)

func (a *App) runtimeConfig(ctx context.Context) (config.Config, error) {
	r, e := a.request(ctx, request{action: requestRuntimeConfig})
	return r.config, e
}
func (a *App) ConfigureSettings(ctx context.Context, s preferences.Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	_, err := a.request(ctx, request{action: requestRuntimeSettings, settings: s})
	return err
}
func (a *App) applySettings(ctx context.Context, s preferences.Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if s.SleepEnabled && a.cfg.GRPCAddress == "" {
		return fmt.Errorf("请先在 LuCI 服务设置中填写 gRPC 地址")
	}
	if a.switchBusy || a.pending != "" || a.healthBusy {
		return fmt.Errorf("操作正在进行，请稍后保存")
	}
	old := a.cfg.Preferences()
	if reflect.DeepEqual(old, s) {
		return nil
	}
	if err := preferences.Save(a.cfg.PreferencesPath(), s); err != nil {
		return fmt.Errorf("保存运行设置: %w", err)
	}
	sleepChanged := old.SleepEnabled != s.SleepEnabled || old.SleepIdle != s.SleepIdle || old.QuietBytes != s.QuietBytes || old.WakeWindow != s.WakeWindow || old.WakeBytes != s.WakeBytes
	probeChanged := old.TestURL != s.TestURL || old.Timeout != s.Timeout
	scheduleChanged := probeChanged || old.Current != s.Current || old.Candidate != s.Candidate || old.Ordinary != s.Ordinary || old.Recovery != s.Recovery || !reflect.DeepEqual(old.RecoverySteps, s.RecoverySteps)
	a.cfg.ApplyPreferences(s)
	level, _ := events.ParseLevel(s.LogLevel)
	a.events.SetLevel(level)
	if setter, ok := a.client.(interface{ SetTestURL(string) }); ok {
		setter.SetTestURL(s.TestURL)
	}
	a.runner.Timeout = a.cfg.Timeout
	now := time.Now()
	if sleepChanged {
		if a.sleeping && !s.SleepEnabled {
			a.wakeFromSleepWithReason(activity.Event{}, "settings")
		}
		a.sleepController.Reconfigure(sleep.Config{Enabled: s.SleepEnabled, IdleAfter: a.cfg.SleepIdleAfter, QuietBytes: s.QuietBytes, WakeWindow: a.cfg.SleepWakeWindow, WakeBytes: s.WakeBytes}, now)
	}
	if scheduleChanged {
		a.generation++
		a.probeCancel()
		a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
		for _, n := range a.store.Nodes {
			n.Plan.InFlight = false
		}
	}
	if old.SleepEnabled != s.SleepEnabled || old.TestURL != s.TestURL {
		a.restartActivity(ctx)
	}
	a.store.Limit = s.HistoryLimit
	for _, n := range a.store.Nodes {
		if len(n.History) > s.HistoryLimit {
			n.History = append(n.History[:0:0], n.History[len(n.History)-s.HistoryLimit:]...)
		}
		if scheduleChanged {
			n.Plan.ClearRecovery()
		}
	}
	if a.phase == "startup" && ((!a.resumeWindow && old.Startup != s.Startup) || (a.resumeWindow && old.Resume != s.Resume)) {
		ids := make([]string, 0, len(a.store.Nodes))
		for _, n := range a.store.Nodes {
			ids = append(ids, n.Info.ID)
		}
		duration := a.cfg.Startup
		if a.resumeWindow {
			duration = a.cfg.Resume
		}
		a.started = now
		a.observeWindow.Begin(duration, ids, now)
	}
	if scheduleChanged && a.phase == "unavailable" {
		for _, n := range a.store.Nodes {
			n.Plan.NormalDue = now.Add(a.cfg.Recovery)
		}
	} else if scheduleChanged {
		a.store.Reschedule(now, a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
	}
	a.revision++
	a.events.Info("settings", "applied", "运行设置已保存并生效", nil)
	return nil
}
