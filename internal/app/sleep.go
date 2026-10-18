package app

import (
	"context"
	"fmt"
	"sing-box-smart/internal/activity"
	"sing-box-smart/internal/sleep"
	"strings"
	"time"
)

func (a *App) handleActivity(event activity.Event) {
	if !a.sleepController.Enabled() {
		return
	}
	if a.sleepController.Observe(activityToSleepEvent(event)) && a.sleeping {
		a.wakeFromSleep(event)
	}
}

func activityToSleepEvent(event activity.Event) sleep.Event {
	return sleep.Event{At: event.At, ID: event.ID, Probe: event.Probe, Active: event.Kind == activity.ConnectionNew, UplinkDelta: event.UplinkDelta, DownlinkDelta: event.DownlinkDelta}
}

func (a *App) enterSleep() {
	if a.sleeping || a.paused || a.pending != "" || a.switchBusy {
		return
	}
	a.sleeping = true
	a.sleepRecords.Start(time.Now(), "idle")
	a.generation++
	a.probeCancel()
	for _, n := range a.store.Nodes {
		n.Plan.InFlight = false
		n.Plan.ClearRecovery()
	}
	a.phase = "sleeping"
	a.revision++
	a.events.Record("sleep", "检测到长时间无用户活动，自动暂停节点检测")
}

func (a *App) wakeFromSleep(event activity.Event) {
	a.wakeFromSleepWithReason(event, "connection")
}
func (a *App) wakeFromSleepWithReason(event activity.Event, reason string) {
	if !a.sleeping {
		return
	}
	now := time.Now()
	source := activitySource(event)
	a.sleepRecords.EndWithSource(now, reason, source)
	a.sleepController.WakeAt(now)
	a.sleeping = false
	a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
	a.revision++
	a.started = now
	a.phase = "startup"
	a.resumeWindow = true
	ids := make([]string, 0, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		ids = append(ids, n.Info.ID)
		n.Plan.InFlight = false
		n.Plan.ClearRecovery()
		n.Plan.NormalDue = time.Time{}
	}
	a.observeWindow.Begin(a.cfg.Resume, ids, now)
	message := "检测到用户活动，恢复检测并开始快速观察"
	if source != "" {
		message = fmt.Sprintf("检测到用户活动（%s），恢复检测并开始快速观察", source)
	}
	if reason == "settings" {
		message = "已关闭自动睡眠，恢复检测并开始快速观察"
	}
	if reason == "manual" {
		message = "手动继续检测，结束睡眠并开始快速观察"
	}
	a.events.Record("sleep", message)
}

// activitySource returns a compact, privacy-conscious label for the first
// real connection that crossed the wake threshold. It intentionally excludes
// URL paths and query strings because the gRPC connection event does not need
// to expose them in the UI.
func activitySource(event activity.Event) string {
	domain := strings.TrimSpace(event.Domain)
	destination := strings.TrimSpace(event.Destination)
	switch {
	case domain != "" && destination != "":
		return domain + " → " + destination
	case domain != "":
		return domain
	case destination != "":
		return destination
	default:
		return ""
	}
}
