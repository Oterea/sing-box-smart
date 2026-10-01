package app

import (
	"context"
	"sing-box-smart/internal/decision"
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
	if a.paused || a.phase == "startup" || !a.healthy || a.pending != "" || a.switchBusy {
		return
	}
	if a.phase == "unavailable" && a.store.AnyAvailable() {
		a.revision++
		a.phase = "normal"
		a.store.Reclassify(a.current)
		a.store.Reschedule(time.Now(), a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
		a.events.Record("phase", "有节点检查成功，恢复正常检查频率")
	}
	target := a.choose()
	if target == "" {
		return
	}
	a.pending = target
	a.pendingKind = "node"
	a.revision++
	a.manual = false
	candidate := a.store.ByID[target]
	v, _ := score.Display(candidate.Metrics)
	a.events.Trace("decision", "current", a.current, "candidate", target, "candidate_score", v, "threshold", a.cfg.SwitchRatio)
	a.startProbe(ctx, candidate, "confirm")
}

func (a *App) apply(ctx context.Context, kind, target, group, value string) {
	if a.paused {
		return
	}
	a.switchBusy = true
	go func() {
		c, cancel := context.WithTimeout(ctx, 2*a.cfg.Timeout)
		defer cancel()
		r := switching.Apply(c, a.client, group, value)
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
	if !c.result.Verified {
		if c.kind == "node" && a.store.ByID[c.result.Actual] != nil {
			a.current = c.result.Actual
			a.store.Reclassify(a.current)
			a.store.Reschedule(time.Now(), a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
		}
		a.events.Record("error", "切换未确认，等待后续重新判断")
		return
	}
	if c.kind == "airport" {
		for _, airport := range a.airports {
			if airport.ID == c.target {
				a.events.Record("switch", "已切换机场 PIN："+airport.Selector)
				a.reset(airport)
				return
			}
		}
	}
	a.current = c.target
	a.store.Reclassify(a.current)
	a.store.Reschedule(time.Now(), a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
	a.events.Record("switch", "已确认选中："+a.store.ByID[c.target].Info.Name)
}
