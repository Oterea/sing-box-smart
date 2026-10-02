package app

import (
	"context"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/recovery"
	"sing-box-smart/internal/score"
	"sing-box-smart/internal/state"
	"time"
)

func (a *App) Run(ctx context.Context) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-a.requests:
			if r.ctx != nil && r.ctx.Err() != nil {
				r.reply <- response{err: r.ctx.Err()}
				continue
			}
			if r.action == "snapshot" {
				r.reply <- response{snapshot: a.snapshot()}
			} else if r.action == "api" {
				r.reply <- response{err: a.configureAPI(r)}
			} else {
				r.reply <- response{err: a.control(ctx, r.action, r.target)}
			}
		case p := <-a.results:
			a.observe(ctx, p)
		case c := <-a.changes:
			a.switched(c)
		case err := <-a.health:
			a.healthBusy = false
			a.revision++
			if err == nil {
				a.healthy = true
				a.events.Record("api", "管理接口恢复，继续检查")
			} else {
				a.lastHealth = time.Now()
			}
		case now := <-ticker.C:
			a.tick(ctx, now)
		}
	}
}

func (a *App) tick(ctx context.Context, now time.Time) {
	if a.paused {
		return
	}
	if !a.healthy {
		if !a.healthBusy && now.Sub(a.lastHealth) >= a.cfg.Recovery {
			a.healthBusy = true
			a.lastHealth = now
			client := a.client
			go func() {
				c, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
				defer cancel()
				err := client.Health(c)
				select {
				case a.health <- err:
				case <-ctx.Done():
				}
			}()
		}
		return
	}
	if a.phase == "startup" && now.Sub(a.started) >= a.cfg.Startup && a.store.AllChecked() {
		a.revision++
		a.phase = "normal"
		if !a.store.AnyAvailable() {
			a.phase = "unavailable"
		}
		a.store.Reclassify(a.current)
		a.store.Reschedule(now, a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
		if a.phase == "unavailable" {
			for _, n := range a.store.Nodes {
				n.Plan.ClearRecovery()
				n.Plan.NormalDue = now.Add(a.cfg.Recovery)
			}
		}
		a.events.Record("phase", "启动观察结束，开始按分数选择")
		a.evaluate(ctx)
	}
	for _, n := range a.store.Nodes {
		if reason := n.Plan.Due(now, a.phase, n.Info.ID == a.current); reason != "" {
			a.startProbe(ctx, n, reason)
		}
	}
}

func (a *App) startProbe(ctx context.Context, n *state.Node, reason string) {
	if a.paused || n.Plan.InFlight {
		return
	}
	n.Plan.InFlight = true
	a.revision++
	n.Plan.Reason = reason
	id, generation := n.Info.ID, a.generation
	probeCtx := a.probeCtx
	runner := a.runner
	go func() {
		p, err := runner.Run(probeCtx, id)
		select {
		case a.results <- observation{id, generation, p, err}:
		case <-probeCtx.Done():
		}
	}()
}

func (a *App) observe(ctx context.Context, o observation) {
	if a.paused || o.generation != a.generation {
		return
	}
	n := a.store.ByID[o.id]
	if n == nil {
		return
	}
	n.Plan.InFlight = false
	if o.err != nil {
		if a.healthy {
			a.events.Record("api", "检查接口异常，暂停切换；不计入节点失败")
		}
		a.healthy = false
		a.revision++
		a.lastHealth = time.Now()
		if !a.switchBusy {
			a.pending = ""
			a.pendingKind = ""
		}
		return
	}
	if o.probe.At.IsZero() {
		o.probe.At = time.Now()
	}
	reason := n.Plan.Reason
	wasCurrent := o.id == a.current
	wasOrdinary := n.Tier == domain.TierOrdinary
	signal := recovery.Signal{}
	if a.phase != "startup" && reason == "normal" && !wasCurrent && wasOrdinary && n.Checks > 0 {
		signal = recovery.Detect(n.History, n.Last, o.probe, a.cfg.RecoveryDropRatio, a.cfg.RecoveryDropMinMS, a.cfg.RecoveryGoodMaxMS)
	}
	a.store.Record(o.id, o.probe)
	a.revision++
	changedTiers := a.store.Reclassify(a.current)
	now := time.Now()
	if signal.Kind == recovery.None && a.phase != "startup" && reason == "normal" && !wasCurrent && wasOrdinary && o.probe.Success && a.store.NearCandidate(o.id, 0.15) {
		signal = recovery.Signal{Kind: recovery.CandidateNear}
	}
	if reason == "recovery" {
		next := a.cfg.Ordinary
		if n.Tier == domain.TierCandidate {
			next = a.cfg.Candidate
		}
		n.Plan.FinishRecovery(now, o.probe.Success, a.cfg.RecoverySteps, next)
		if n.Plan.RecoveryStep == 0 {
			a.events.Record("recovery", n.Info.Name+"：结束恢复复查")
		}
	} else if a.phase == "unavailable" {
		n.Plan.ClearRecovery()
		n.Plan.NormalDue = now.Add(a.cfg.Recovery)
	} else if !wasCurrent && reason == "normal" && signal.Kind != recovery.None && wasOrdinary {
		if n.Plan.StartRecovery(now, a.cfg.RecoverySteps) {
			a.events.Record("recovery", n.Info.Name+"：发现"+recoveryLabel(signal.Kind)+"，3 秒后开始恢复复查")
		}
	} else {
		n.Plan.ClearRecovery()
		a.store.RescheduleNode(now, n, a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
	}
	for _, id := range changedTiers {
		if id == o.id {
			continue
		}
		other := a.store.ByID[id]
		if other != nil && !other.Plan.InFlight && other.Plan.RecoveryStep == 0 {
			a.store.RescheduleNode(now, other, a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
		}
	}
	value, overflow := score.Display(n.Metrics)
	var loggedScore any = "—"
	if value != nil {
		loggedScore = *value
	} else if overflow {
		loggedScore = "∞"
	}
	a.events.Trace("probe", "node", o.id, "reason", n.Plan.Reason, "success", o.probe.Success, "delay_ms", o.probe.DelayMS, "score", loggedScore, "score_overflow", overflow, "L", n.Metrics.L, "S", n.Metrics.S, "N", n.Metrics.N, "f", n.Metrics.F)
	if !a.healthy {
		return
	}
	if a.phase != "startup" {
		if !a.store.AnyAvailable() && a.phase != "unavailable" {
			a.revision++
			a.phase = "unavailable"
			for _, x := range a.store.Nodes {
				x.Plan.ClearRecovery()
				x.Plan.NormalDue = now.Add(a.cfg.Recovery)
			}
			a.events.Record("unavailable", "当前机场暂时全部不可用，保留原选择，每 3 秒重试")
		}
	}
	if a.pendingKind == "node" && a.pending == o.id && !a.switchBusy {
		if o.probe.Success && (a.manual || a.choose() == o.id) {
			a.apply(ctx, "node", o.id, a.airport.Selector, o.id)
		} else {
			a.events.Record("decision", "取消本次切换：目标检查失败或更新后分差不足")
			a.pending = ""
			a.pendingKind = ""
		}
		return
	}
	a.evaluate(ctx)
}
