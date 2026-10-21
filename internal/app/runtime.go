package app

import (
	"context"
	"fmt"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/recovery"
	"sing-box-smart/internal/score"
	"sing-box-smart/internal/state"
	"time"
)

func (a *App) Run(ctx context.Context) {
	defer func() { _ = a.longTerm.Flush() }()
	a.restartActivity(ctx)
	defer a.stopActivity()
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
			if r.action == "runtime-config" {
				r.reply <- response{config: a.cfg}
			} else if r.action == "node-stability" {
				result, ok := a.store.NodeStability(r.target, time.Now(), a.cfg.DelayGreen, a.cfg.DelayYellow, r.since)
				if !ok {
					r.reply <- response{err: fmt.Errorf("节点不存在")}
				} else {
					r.reply <- response{stability: result}
				}
			} else if r.action == "node-longterm" {
				if n := a.store.ByID[r.target]; n == nil {
					r.reply <- response{err: fmt.Errorf("节点不存在")}
				} else {
					r.reply <- response{longTerm: a.longTerm.Query(a.airport.ID, r.target, time.Now(), float64(a.cfg.DelayYellow))}
				}
			} else if r.action == "runtime-settings" {
				r.reply <- response{err: a.applySettings(ctx, r.settings)}
			} else if r.action == "snapshot" {
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
		case selection := <-a.selection:
			a.selectionSyncBusy = false
			a.applySelectionObservation(selection)
		case event := <-a.activityEvents:
			a.handleActivity(event)
		case now := <-ticker.C:
			a.tick(ctx, now)
		}
	}
}

func (a *App) tick(ctx context.Context, now time.Time) {
	if a.confirmation.Expired(now) {
		a.cancelNodeConfirmation("确认超时，取消本次切换")
	}
	if a.sleepController.Tick(now, a.paused || a.pending != "" || a.switchBusy) {
		a.enterSleep()
	}
	if !a.selectionSyncBusy && (a.lastSelectionSync.IsZero() || now.Sub(a.lastSelectionSync) >= selectionSyncInterval) {
		a.lastSelectionSync = now
		a.startSelectionSync(ctx)
	}
	if a.paused || a.sleeping {
		return
	}
	if !a.healthy {
		if !a.healthBusy && now.Sub(a.lastHealth) >= a.cfg.Recovery {
			a.healthBusy = true
			a.lastHealth = now
			client, timeout := a.client, a.cfg.Timeout
			go func() {
				c, cancel := context.WithTimeout(ctx, timeout)
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
	if a.phase == "startup" && a.observeWindow.Ready(now) {
		resumeWindow := a.resumeWindow
		a.revision++
		a.phase = "normal"
		a.observeWindow.Reset()
		a.resumeWindow = false
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
		if resumeWindow {
			a.events.Record("phase", "恢复观察结束，开始按分数选择")
		} else {
			a.events.Record("phase", "启动观察结束，开始按分数选择")
		}
		a.evaluate(ctx)
	}
	for _, n := range a.store.Nodes {
		if reason := n.Plan.Due(now, a.phase, n.Info.ID == a.current); reason != "" {
			a.startProbe(ctx, n, reason)
		}
	}
}

func (a *App) cancelNodeConfirmation(message string) {
	if !a.confirmation.Active() {
		return
	}
	a.confirmation.Clear()
	a.pending = ""
	a.pendingKind = ""
	a.pendingManual = false
	a.revision++
	a.events.Record("decision", message)
}

func (a *App) startProbe(ctx context.Context, n *state.Node, reason string) {
	if a.paused || a.sleeping || n.Plan.InFlight {
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
	if a.paused || a.sleeping || o.generation != a.generation {
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
			a.confirmation.Clear()
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
	if err := a.longTerm.Record(a.airport.ID, o.id, n.Info.Name, o.probe); err != nil {
		a.events.Warn("stability", "long_term_save", "长期稳定性数据保存失败", map[string]any{"error": err.Error()})
	}
	var delay *float64
	if o.probe.Success {
		value := o.probe.DelayMS
		delay = &value
	}
	a.stabilityHub.Publish(domain.StabilityUpdate{NodeID: o.id, Sample: domain.StabilitySample{At: o.probe.At, Success: o.probe.Success, DelayMS: delay, Error: o.probe.Error}, Summary: n.Stability.Summary(time.Now(), float64(a.cfg.DelayGreen), float64(a.cfg.DelayYellow))})
	if a.phase == "startup" {
		a.observeWindow.Complete(o.id)
	}
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
			a.events.Record("recovery", fmt.Sprintf("%s：发现%s，%.0f 秒后开始恢复复查", n.Info.Name, recoveryLabel(signal.Kind), a.cfg.RecoverySteps[0].Seconds()))
		}
	} else if reason == "confirm" {
		// A confirmation batch owns the target until it completes. Keep the
		// ordinary schedule behind it so the next confirmation is the only
		// probe that can start for this node.
		n.Plan.ClearRecovery()
		n.Plan.NormalDue = now.Add(a.cfg.Candidate)
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
	a.events.Trace("probe", "node", o.id, "reason", n.Plan.Reason, "success", o.probe.Success, "delay_ms", o.probe.DelayMS, "score", loggedScore, "score_overflow", overflow, "L", n.Metrics.L, "S", n.Metrics.S, "N", n.Metrics.N, "f", n.Metrics.F, "jitter_ms", n.Metrics.J, "jitter_penalty", score.JitterPenalty(n.Metrics), "success_samples", n.Metrics.Successes)
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
			a.events.Record("unavailable", fmt.Sprintf("当前机场暂时全部不可用，保留原选择，每 %.0f 秒重试", a.cfg.Recovery.Seconds()))
		}
	}
	if a.phase == "unavailable" && a.store.AnyAvailable() {
		a.phase = "normal"
		a.revision++
		a.store.Reclassify(a.current)
		a.store.Reschedule(now, a.current, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
		a.events.Record("phase", "有节点检查成功，恢复正常检查频率")
	}
	if a.pendingKind == "node" && a.pending == o.id && !a.switchBusy {
		if reason == "confirm" && a.confirmation.Active() && a.confirmation.Target == o.id {
			a.confirmation.Complete()
			if a.confirmation.Completed < a.confirmation.Attempts {
				a.startProbe(ctx, n, "confirm")
				return
			}
			a.confirmation.Clear()
			if o.probe.Success && (a.pendingManual || (a.selectionMode == domain.SelectionAuto && a.controlActive && a.choose() == o.id)) {
				a.apply(ctx, "node", o.id, a.airport.Selector, o.id)
			} else {
				a.events.Record("decision", "取消本次切换：确认探测未通过或更新后分差不足")
				a.pending = ""
				a.pendingKind = ""
				a.pendingManual = false
			}
			return
		}
		if o.probe.Success && (a.pendingManual || (a.selectionMode == domain.SelectionAuto && a.controlActive && a.choose() == o.id)) {
			a.apply(ctx, "node", o.id, a.airport.Selector, o.id)
		} else {
			a.events.Record("decision", "取消本次切换：目标检查失败或更新后分差不足")
			a.pending = ""
			a.pendingKind = ""
		}
		return
	}
	// A result from a confirmation that was canceled by pause, manual mode,
	// configuration reset, or the total deadline may still be queued. Score it
	// normally, but do not let that stale result immediately start a new switch.
	if reason == "confirm" {
		return
	}
	a.evaluate(ctx)
}
