// Package app composes modules and serializes state changes through one event loop.
package app

import (
	"context"
	"fmt"
	"log"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/decision"
	"sing-box-smart/internal/discovery"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/gateway"
	"sing-box-smart/internal/probe"
	"sing-box-smart/internal/recovery"
	"sing-box-smart/internal/score"
	"sing-box-smart/internal/state"
	"sing-box-smart/internal/switching"
	"time"
)

type request struct {
	action, target string
	reply          chan response
}
type response struct {
	snapshot domain.Snapshot
	err      error
}
type observation struct {
	id         string
	generation int
	probe      domain.Probe
	err        error
}
type changed struct {
	kind, target string
	result       switching.Result
}
type App struct {
	cfg                                     config.Config
	client                                  gateway.Client
	runner                                  probe.Runner
	airports                                []domain.Airport
	airport                                 domain.Airport
	store                                   *state.Store
	events                                  events.Recorder
	phase, current, pending, pendingKind    string
	manual, switchBusy, healthy, healthBusy bool
	started, lastHealth                     time.Time
	generation                              int
	requests                                chan request
	results                                 chan observation
	changes                                 chan changed
	health                                  chan error
	rootCtx, probeCtx                       context.Context
	probeCancel                             context.CancelFunc
}

func New(ctx context.Context, cfg config.Config, c gateway.Client, logger *log.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	airports, err := discovery.Load(ctx, c)
	if err != nil {
		return nil, err
	}
	probeCtx, probeCancel := context.WithCancel(ctx)
	a := &App{cfg: cfg, client: c, runner: probe.Runner{Client: c, Timeout: cfg.Timeout}, airports: airports, events: events.Recorder{Logger: logger}, healthy: true, requests: make(chan request), results: make(chan observation, 128), changes: make(chan changed, 1), health: make(chan error, 1), rootCtx: ctx, probeCtx: probeCtx, probeCancel: probeCancel}
	a.reset(airports[0])
	return a, nil
}
func (a *App) reset(airport domain.Airport) {
	if a.probeCancel != nil {
		a.probeCancel()
	}
	a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
	a.airport = airport
	a.store = state.New(airport.Nodes, a.cfg.HistoryLimit)
	a.generation++
	a.started = time.Now()
	a.phase = "startup"
	a.current = ""
	a.pending = ""
	a.pendingKind = ""
	a.switchBusy = false
	a.events.Record("startup", airport.Name+"：开始约 10 秒启动检查，期间不选择节点")
}
func (a *App) request(ctx context.Context, r request) (response, error) {
	r.reply = make(chan response, 1)
	select {
	case a.requests <- r:
	case <-ctx.Done():
		return response{}, ctx.Err()
	}
	select {
	case result := <-r.reply:
		return result, result.err
	case <-ctx.Done():
		return response{}, ctx.Err()
	}
}
func (a *App) Snapshot(ctx context.Context) (domain.Snapshot, error) {
	r, e := a.request(ctx, request{action: "snapshot"})
	return r.snapshot, e
}
func (a *App) Control(ctx context.Context, action, target string) error {
	_, err := a.request(ctx, request{action: action, target: target})
	return err
}
func (a *App) Run(ctx context.Context) {
	ticker := time.NewTicker(25 * time.Millisecond)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case r := <-a.requests:
			if r.action == "snapshot" {
				r.reply <- response{snapshot: a.snapshot()}
			} else {
				r.reply <- response{err: a.control(ctx, r.action, r.target)}
			}
		case p := <-a.results:
			a.observe(ctx, p)
		case c := <-a.changes:
			a.switched(c)
		case err := <-a.health:
			a.healthBusy = false
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
func (a *App) snapshot() domain.Snapshot {
	return domain.Snapshot{Mode: a.cfg.Mode, Phase: a.phase, AirportID: a.airport.ID, Airports: a.airports, CurrentID: a.current, PendingID: a.pending, APIHealthy: a.healthy, StartedAt: a.started, Now: time.Now(), StartupSeconds: a.cfg.Startup.Seconds(), Nodes: a.store.Views(a.current, a.phase), Events: append([]domain.Event{}, a.events.Recent...)}
}
func (a *App) control(ctx context.Context, action, target string) error {
	if action == "recheck" {
		for _, n := range a.store.Nodes {
			n.Plan.ClearRecovery()
			n.Plan.NormalDue = time.Now()
		}
		a.events.Record("manual", "已安排当前机场全部节点重新检查")
		return nil
	}
	if !a.healthy {
		return fmt.Errorf("管理接口暂不可用")
	}
	if a.pending != "" || a.switchBusy {
		return fmt.Errorf("已有切换正在处理中")
	}
	switch action {
	case "node":
		if a.phase == "startup" {
			return fmt.Errorf("启动检查尚未结束")
		}
		if a.store.ByID[target] == nil {
			return fmt.Errorf("节点不属于当前机场")
		}
		if target == a.current {
			return nil
		}
		a.pending = target
		a.pendingKind = "node"
		a.manual = true
		a.events.Record("manual", "手动选择：先检查目标节点")
		a.startProbe(ctx, a.store.ByID[target], "confirm")
	case "airport":
		for _, airport := range a.airports {
			if airport.ID == target {
				if target == a.airport.ID {
					return nil
				}
				a.pending = target
				a.pendingKind = "airport"
				a.apply(ctx, "airport", target, "proxy", airport.Selector)
				return nil
			}
		}
		return fmt.Errorf("未知机场")
	default:
		return fmt.Errorf("未知操作")
	}
	return nil
}
func (a *App) tick(ctx context.Context, now time.Time) {
	if !a.healthy {
		if !a.healthBusy && now.Sub(a.lastHealth) >= a.cfg.Recovery {
			a.healthBusy = true
			a.lastHealth = now
			go func() {
				c, cancel := context.WithTimeout(ctx, a.cfg.Timeout)
				defer cancel()
				err := a.client.Health(c)
				select {
				case a.health <- err:
				case <-ctx.Done():
				}
			}()
		}
		return
	}
	if a.phase == "startup" && now.Sub(a.started) >= a.cfg.Startup && a.store.AllChecked() {
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
	if n.Plan.InFlight {
		return
	}
	n.Plan.InFlight = true
	n.Plan.Reason = reason
	id, generation := n.Info.ID, a.generation
	probeCtx := a.probeCtx
	go func() {
		p, err := a.runner.Run(probeCtx, id)
		select {
		case a.results <- observation{id, generation, p, err}:
		case <-probeCtx.Done():
		}
	}()
}
func (a *App) observe(ctx context.Context, o observation) {
	if o.generation != a.generation {
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
	signal := recovery.Signal{}
	if a.phase != "startup" && reason == "normal" && !wasCurrent && n.Tier == domain.TierOrdinary && n.Checks > 0 {
		signal = recovery.Detect(n.History, n.Last, o.probe, a.cfg.RecoveryDropRatio, a.cfg.RecoveryDropMinMS)
	}
	a.store.Record(o.id, o.probe)
	changedTiers := a.store.Reclassify(a.current)
	now := time.Now()
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
	} else if !wasCurrent && reason == "normal" && signal.Kind != recovery.None && n.Tier == domain.TierOrdinary {
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

func recoveryLabel(kind recovery.Kind) string {
	if kind == recovery.FailureToGood {
		return "失败后的成功"
	}
	if kind == recovery.LatencyDrop {
		return "延迟明显下降"
	}
	return "恢复迹象"
}
func (a *App) choose() string {
	nodes := make([]decision.Candidate, 0, len(a.store.Nodes))
	for _, n := range a.store.Nodes {
		v, ok := score.Value(n.Metrics)
		nodes = append(nodes, decision.Candidate{ID: n.Info.ID, Score: v, HasScore: ok, Available: n.Checks > 0 && n.Last.Success})
	}
	return decision.Choose(nodes, a.current, a.cfg.SwitchRatio)
}
func (a *App) evaluate(ctx context.Context) {
	if a.phase == "startup" || !a.healthy || a.pending != "" || a.switchBusy {
		return
	}
	if a.phase == "unavailable" && a.store.AnyAvailable() {
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
	a.manual = false
	candidate := a.store.ByID[target]
	v, _ := score.Display(candidate.Metrics)
	a.events.Trace("decision", "current", a.current, "candidate", target, "candidate_score", v, "threshold", a.cfg.SwitchRatio)
	a.startProbe(ctx, candidate, "confirm")
}
func (a *App) apply(ctx context.Context, kind, target, group, value string) {
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
