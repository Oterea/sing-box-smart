package app

import (
	"context"
	"sing-box-smart/internal/decision"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/score"
	"sing-box-smart/internal/switching"
	"time"
)

func (a *App) choose() string {
	nodes := make([]decision.Candidate, 0, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		v, ok := score.Value(n.Metrics)
		nodes = append(nodes, decision.Candidate{ID: n.Info.ID, Score: v, HasScore: ok, Available: n.Checks > 0 && n.Last.Success})
	}
	return decision.Choose(nodes, a.current, a.cfg.SwitchRatio)
}

func (a *App) evaluate(ctx context.Context) {
	if a.paused || a.selectionMode == domain.SelectionManual || !a.controlActive || !a.selectionInitialized || a.phase == "startup" || !a.healthy || a.pending != "" || a.switchBusy {
		return
	}
	target := a.choose()
	if target == "" {
		return
	}
	a.pending = target
	a.pendingKind = "node"
	a.pendingManual = false
	a.selectionEpoch++
	a.revision++
	candidate := a.store.ByID[target]
	v, overflow := score.Display(candidate.Metrics)
	var loggedScore any = "—"
	if v != nil {
		loggedScore = *v
	} else if overflow {
		loggedScore = "∞"
	}
	a.events.Trace("decision", "current", a.current, "candidate", target, "candidate_score", loggedScore, "threshold", a.cfg.SwitchRatio)
	a.startProbe(ctx, candidate, "confirm")
}

func (a *App) apply(ctx context.Context, kind, target, group, value string) {
	if a.paused {
		return
	}
	a.switchBusy = true
	client, root, selector, current, manual, timeout := a.client, a.groupRoot, a.airport.Selector, a.current, a.pendingManual, a.cfg.Timeout
	go func() {
		c, cancel := context.WithTimeout(ctx, 2*timeout)
		defer cancel()
		var r switching.Result
		if kind == "node" {
			r = switching.ApplyGuarded(c, client, root, selector, current, value, manual)
		} else {
			r = switching.Apply(c, client, group, value)
		}
		select {
		case a.changes <- changed{kind, target, r}:
		case <-ctx.Done():
		}
	}()
}

func (a *App) switched(c changed) {
	a.revision++
	a.switchBusy = false
	a.pending = ""
	a.pendingKind = ""
	a.pendingManual = false
	a.selectionEpoch++
	if c.result.Superseded {
		a.setSelectionMode(domain.SelectionManual)
		a.applySelectionObservation(selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: c.result.RootActual, node: c.result.Actual})
		a.lastSelectionSync = time.Time{}
		a.events.Record("manual", "切换前发现 sing-box 已被外部修改，取消 smart 写入")
		return
	}
	if !c.result.Verified {
		if c.kind == "node" && a.store.ByID[c.result.Actual] != nil {
			a.setCurrentNode(c.result.Actual)
			a.selectionInitialized = true
			a.setSelectionMode(domain.SelectionManual)
		}
		a.events.Record("error", "切换未确认，等待后续重新判断")
		return
	}
	if c.kind == "airport" {
		for _, airport := range a.airports {
			if airport.ID == c.target {
				a.events.Record("switch", "已切换策略组："+airport.Selector)
				a.reset(airport)
				a.rootSelection = airport.Selector
				a.controlActive = true
				a.lastSelectionSync = time.Time{}
				return
			}
		}
	}
	a.selectionInitialized = true
	a.setCurrentNode(c.target)
	a.events.Record("switch", "已确认选中："+a.store.ByID[c.target].Info.Name)
}
