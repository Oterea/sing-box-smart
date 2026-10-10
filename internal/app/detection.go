package app

import (
	"context"
	"sing-box-smart/internal/scan"
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
	a.generation++
	a.probeCancel()
	a.refresh.Reset()
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
	a.paused = false
	a.revision++
	now := time.Now()
	ids := make([]string, 0, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		n.Plan.InFlight = false
		n.Plan.ClearRecovery()
		n.Plan.NormalDue = time.Time{}
		ids = append(ids, n.Info.ID)
	}
	a.refresh.Begin(scan.Resume, ids, now)
	a.phase = "refresh"
	a.events.Record("manual", "继续检测，先快速检查当前机场全部节点，完成后恢复正常调度")
	if !a.refresh.Active() {
		a.finishRefresh(a.rootCtx, now)
	}
}
