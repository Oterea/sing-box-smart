package app

import (
	"context"
	"sing-box-smart/internal/activity"
	"sing-box-smart/internal/sleep"
	"time"
)

func (a *App) handleActivity(event activity.Event) {
	if !a.sleepController.Enabled() {
		return
	}
	if a.sleepController.Observe(activityToSleepEvent(event)) && a.sleeping {
		a.wakeFromSleep()
	}
}

func activityToSleepEvent(event activity.Event) sleep.Event {
	active := event.Kind == activity.ConnectionNew || event.UplinkDelta > 0 || event.DownlinkDelta > 0
	return sleep.Event{At: event.At, Probe: event.Probe, Active: active}
}

func (a *App) enterSleep() {
	if a.sleeping || a.paused || a.pending != "" || a.switchBusy {
		return
	}
	a.sleeping = true
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

func (a *App) wakeFromSleep() {
	if !a.sleeping {
		return
	}
	a.sleepController.Wake()
	a.sleepController.Observe(sleep.Event{At: time.Now(), Active: true})
	a.sleeping = false
	a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
	a.revision++
	a.started = time.Now()
	a.phase = "startup"
	a.resumeWindow = true
	ids := make([]string, 0, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		ids = append(ids, n.Info.ID)
		n.Plan.InFlight = false
		n.Plan.ClearRecovery()
		n.Plan.NormalDue = time.Time{}
	}
	a.observeWindow.Begin(a.cfg.Resume, ids, time.Now())
	a.events.Record("sleep", "检测到用户活动，恢复检测并开始快速观察")
}
