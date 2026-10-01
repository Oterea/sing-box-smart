package app

import (
	"context"
	"time"
)

// pauseDetection invalidates outstanding observations before cancellation, so
// even results already queued cannot change scores or initiate a switch.
func (a *App) pauseDetection() {
	if a.paused {
		return
	}
	a.paused = true
	a.revision++
	a.pausedAt = time.Now()
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
	if !a.paused {
		return
	}
	a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
	if a.phase == "startup" {
		a.started = a.started.Add(time.Since(a.pausedAt))
	}
	a.paused = false
	a.revision++
	// Refresh every node once rather than choosing from stale pre-pause scores.
	for _, n := range a.store.Nodes {
		n.Plan.NormalDue = time.Now()
	}
	a.events.Record("manual", "继续检测，重新检查当前策略组节点")
}
