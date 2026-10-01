// Package app composes modules and serializes state changes through one event loop.
package app

import (
	"context"
	"fmt"
	"log"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/discovery"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/gateway"
	"sing-box-smart/internal/probe"
	"sing-box-smart/internal/real"
	"sing-box-smart/internal/recovery"
	"sing-box-smart/internal/settings"
	"sing-box-smart/internal/state"
	"sing-box-smart/internal/switching"
	"time"
)

type request struct {
	action, target string
	root, pattern  string
	client         gateway.Client
	airports       []domain.Airport
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
	apiAddress, groupRoot, groupPattern     string
	client                                  gateway.Client
	runner                                  probe.Runner
	airports                                []domain.Airport
	airport                                 domain.Airport
	store                                   *state.Store
	events                                  events.Recorder
	phase, current, pending, pendingKind    string
	manual, switchBusy, healthy, healthBusy bool
	paused                                  bool
	pausedAt                                time.Time
	started, lastHealth                     time.Time
	generation                              int
	revision                                uint64
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
	a.apiAddress, a.groupRoot, a.groupPattern = cfg.API, cfg.Root, cfg.Pattern
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
	a.revision++
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
func (a *App) prepareAPI(ctx context.Context, api string) (gateway.Client, []domain.Airport, string, error) {
	if a.cfg.Mode != "real" {
		return nil, nil, "", fmt.Errorf("模拟模式不支持修改真实 API")
	}
	api, err := settings.NormalizeAPI(api)
	if err != nil {
		return nil, nil, "", err
	}
	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	cfg := a.cfg
	cfg.API, cfg.Root, cfg.Pattern = api, snapshot.GroupRoot, snapshot.GroupPattern
	client := real.New(cfg)
	airports, err := discovery.Load(ctx, client)
	if err != nil {
		return nil, nil, "", err
	}
	return client, airports, api, nil
}
func (a *App) TestAPI(ctx context.Context, api string) error {
	_, _, _, err := a.prepareAPI(ctx, api)
	return err
}
func (a *App) ConfigureAPI(ctx context.Context, api string) error {
	client, airports, api, err := a.prepareAPI(ctx, api)
	if err != nil {
		return err
	}
	_, err = a.request(ctx, request{action: "api", target: api, client: client, airports: airports})
	return err
}
func (a *App) ConfigureFilters(ctx context.Context, root, pattern string) error {
	if root == "" {
		root = "proxy"
	}
	if _, err := settings.NormalizePattern(pattern); err != nil {
		return err
	}
	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		return err
	}
	cfg := a.cfg
	cfg.Root = root
	cfg.Pattern = pattern
	cfg.API = snapshot.APIAddress
	client := real.New(cfg)
	airports, err := discovery.Load(ctx, client)
	if err != nil {
		return err
	}
	_, err = a.request(ctx, request{action: "api", target: snapshot.APIAddress, root: root, pattern: pattern, client: client, airports: airports})
	return err
}

func (a *App) snapshot() domain.Snapshot {
	return domain.Snapshot{Revision: a.revision, Paused: a.paused, APIAddress: a.apiAddress, GroupRoot: a.groupRoot, GroupPattern: a.groupPattern, Mode: a.cfg.Mode, Phase: a.phase, AirportID: a.airport.ID, Airports: a.airports, CurrentID: a.current, PendingID: a.pending, APIHealthy: a.healthy, StartedAt: a.started, Now: time.Now(), StartupSeconds: a.cfg.Startup.Seconds(), Nodes: a.store.Views(a.current, a.phase), Events: append([]domain.Event{}, a.events.Recent...)}
}

func recoveryLabel(kind recovery.Kind) string {
	if kind == recovery.FailureToGood {
		return "失败后的成功"
	}
	if kind == recovery.LatencyDrop {
		return "延迟明显下降"
	}
	if kind == recovery.CandidateNear {
		return "分数接近候选边界"
	}
	return "恢复迹象"
}
