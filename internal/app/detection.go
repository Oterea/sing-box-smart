package app

import (
	"context"
	"sing-box-smart/internal/sleep"
	"time"
)

// pauseDetection invalidates outstanding observations before cancellation, so
// even results already queued cannot change scores or initiate a switch.
func (a *App) pauseDetection() {
	if a.paused {
		return
	}
	a.paused = true
	if a.sleeping {
		a.sleeping = false
		a.sleepController.Wake()
	}
	a.revision++
	a.generation++
	a.probeCancel()
	for _, n := range a.store.Nodes {
		n.Plan.InFlight = false
		n.Plan.ClearRecovery()
	}
	if !a.switchBusy {
		a.pending = ""
		a.pendingKind = ""
	}
	a.events.Record("manual", "已暂停检测和自动切换")
}
func (a *App) resumeDetection() {
	if a.sleeping {
		a.wakeFromSleep()
		return
	}
	if !a.paused {
		return
	}
	a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
	a.sleepController.Observe(sleep.Event{At: time.Now(), Active: true})
	a.paused = false
	a.revision++
	now := time.Now()
	a.started = now
	a.phase = "startup"
	a.resumeWindow = true
	ids := make([]string, 0, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		ids = append(ids, n.Info.ID)
	}
	a.observeWindow.Begin(a.cfg.Resume, ids, now)
	for _, n := range a.store.Nodes {
		n.Plan.InFlight = false
		n.Plan.ClearRecovery()
		n.Plan.NormalDue = time.Time{}
	}
	a.events.Record("manual", "继续检测，开始约 5 秒快速观察，期间不自动切换")
}
