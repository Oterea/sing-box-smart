// Package app composes modules and serializes state changes through one event loop.
package app

import (
	"context"
	"log"
	"sing-box-smart/internal/activity"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/discovery"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/gateway"
	"sing-box-smart/internal/observewindow"
	"sing-box-smart/internal/probe"
	"sing-box-smart/internal/recovery"
	"sing-box-smart/internal/sleep"
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
	ctx            context.Context
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
	cfg                                    config.Config
	apiAddress, groupRoot, groupPattern    string
	client                                 gateway.Client
	runner                                 probe.Runner
	airports                               []domain.Airport
	airport                                domain.Airport
	store                                  *state.Store
	events                                 events.Recorder
	phase, current, pending, pendingKind   string
	selectionMode                          domain.SelectionMode
	selectionInitialized                   bool
	rootSelection                          string
	controlActive                          bool
	selectionEpoch                         uint64
	pendingManual                          bool
	switchBusy, healthy, healthBusy        bool
	selectionSyncBusy                      bool
	paused                                 bool
	sleeping                               bool
	activityEvents                         chan activity.Event
	sleepController                        *sleep.Controller
	resumeWindow                           bool
	observeWindow                          observewindow.Window
	started, lastHealth, lastSelectionSync time.Time
	generation                             int
	revision                               uint64
	requests                               chan request
	results                                chan observation
	changes                                chan changed
	health                                 chan error
	selection                              chan selectionObservation
	rootCtx, probeCtx                      context.Context
	probeCancel                            context.CancelFunc
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
	a := &App{cfg: cfg, client: c, runner: probe.Runner{Client: c, Timeout: cfg.Timeout}, airports: airports, events: events.Recorder{Logger: logger}, healthy: true, selectionMode: domain.SelectionAuto, requests: make(chan request), results: make(chan observation, 128), changes: make(chan changed, 1), health: make(chan error, 1), selection: make(chan selectionObservation, 1), activityEvents: make(chan activity.Event, 256), sleepController: sleep.New(sleep.Config{Enabled: cfg.SleepEnabled, IdleAfter: cfg.SleepIdleAfter}, time.Now()), rootCtx: ctx, probeCtx: probeCtx, probeCancel: probeCancel}
	a.apiAddress, a.groupRoot, a.groupPattern = cfg.API, cfg.Root, cfg.Pattern
	a.reset(airports[0])
	return a, nil
}

// ActivitySink is the single input for automatic sleep monitoring.  The
// application event loop remains the owner of sleeping and probe scheduling.
func (a *App) ActivitySink() chan<- activity.Event { return a.activityEvents }
func (a *App) reset(airport domain.Airport) {
	if a.probeCancel != nil {
		a.probeCancel()
	}
	a.probeCtx, a.probeCancel = context.WithCancel(a.rootCtx)
	now := time.Now()
	a.airport = airport
	a.store = state.New(airport.Nodes, a.cfg.HistoryLimit)
	ids := make([]string, 0, len(airport.Nodes))
	for _, n := range airport.Nodes {
		ids = append(ids, n.ID)
	}
	a.observeWindow.Begin(a.cfg.Startup, ids, now)
	a.resumeWindow = false
	a.sleeping = false
	a.sleepController = sleep.New(sleep.Config{Enabled: a.cfg.SleepEnabled, IdleAfter: a.cfg.SleepIdleAfter}, now)
	a.generation++
	a.revision++
	a.started = now
	a.phase = "startup"
	a.current = ""
	a.pending = ""
	a.pendingKind = ""
	a.switchBusy = false
	a.selectionInitialized = false
	a.selectionEpoch++
	a.pendingManual = false
	a.events.Record("startup", airport.Name+"：开始约 10 秒启动检查，期间不选择节点")
}
func (a *App) request(ctx context.Context, r request) (response, error) {
	r.ctx = ctx
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
func (a *App) snapshot() domain.Snapshot {
	var lastActivity *time.Time
	if t := a.sleepController.LastActivity(); !t.IsZero() {
		v := t
		lastActivity = &v
	}
	return domain.Snapshot{Revision: a.revision, Paused: a.paused, Sleeping: a.sleeping, SleepEnabled: a.sleepController.Enabled(), LastActivity: lastActivity, APIAddress: a.apiAddress, GroupRoot: a.groupRoot, GroupPattern: a.groupPattern, Mode: a.cfg.Mode, SelectionMode: a.selectionMode, RootSelection: a.rootSelection, ControlActive: a.controlActive, Phase: a.phase, AirportID: a.airport.ID, Airports: a.airports, CurrentID: a.current, PendingID: a.pending, APIHealthy: a.healthy, StartedAt: a.started, Now: time.Now(), StartupSeconds: a.cfg.Startup.Seconds(), ObservationSeconds: a.observeWindow.Duration().Seconds(), Nodes: a.store.Views(a.current, a.phase), Events: append([]domain.Event{}, a.events.Recent...)}
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
