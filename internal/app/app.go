// Package app composes modules and serializes state changes through one event loop.
package app

import (
	"context"
	"fmt"
	"log"
	"path/filepath"
	"sing-box-smart/internal/activity"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/discovery"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/events"
	"sing-box-smart/internal/gateway"
	"sing-box-smart/internal/observewindow"
	"sing-box-smart/internal/preferences"
	"sing-box-smart/internal/probe"
	"sing-box-smart/internal/recovery"
	"sing-box-smart/internal/sleep"
	"sing-box-smart/internal/stability"
	"sing-box-smart/internal/state"
	"sing-box-smart/internal/switching"
	"time"
)

type request struct {
	settings       preferences.Settings
	action, target string
	root, pattern  string
	since          time.Time
	client         gateway.Client
	airports       []domain.Airport
	reply          chan response
	ctx            context.Context
}
type response struct {
	config    config.Config
	snapshot  domain.Snapshot
	stability domain.NodeStability
	longTerm  domain.LongTermStability
	err       error
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
	activityCancel                         context.CancelFunc
	activityDone                           chan struct{}
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
	confirmation                           switching.Confirmation
	switchBusy, healthy, healthBusy        bool
	selectionSyncBusy                      bool
	paused                                 bool
	sleeping                               bool
	activityEvents                         chan activity.Event
	sleepController                        *sleep.Controller
	sleepRecords                           *sleep.Recorder
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
	stabilityHub                           *stability.Hub
	longTerm                               *stability.LongTerm
}

func New(ctx context.Context, cfg config.Config, c gateway.Client, logger *log.Logger) (*App, error) {
	if err := cfg.Validate(); err != nil {
		return nil, err
	}
	minLevel, err := events.ParseLevel(cfg.LogLevel)
	if err != nil {
		return nil, err
	}
	longTerm, err := stability.NewLongTerm(filepath.Join(cfg.LogDir, "stability-summary.json"))
	if err != nil {
		return nil, err
	}
	airports, err := discovery.Load(ctx, c)
	if err != nil {
		return nil, err
	}
	probeCtx, probeCancel := context.WithCancel(ctx)
	a := &App{cfg: cfg, client: c, runner: probe.Runner{Client: c, Timeout: cfg.Timeout}, airports: airports, events: events.Recorder{Logger: logger, MinLevel: minLevel}, healthy: true, selectionMode: domain.SelectionAuto, requests: make(chan request), results: make(chan observation, 128), changes: make(chan changed, 1), selection: make(chan selectionObservation, 1), activityEvents: make(chan activity.Event, 256), sleepController: sleep.New(sleep.Config{Enabled: cfg.SleepEnabled, IdleAfter: cfg.SleepIdleAfter, QuietBytes: cfg.SleepQuietBytes, WakeWindow: cfg.SleepWakeWindow, WakeBytes: cfg.SleepWakeBytes}, time.Now()), sleepRecords: sleep.NewRecorder(3), rootCtx: ctx, probeCtx: probeCtx, probeCancel: probeCancel, longTerm: longTerm}
	a.stabilityHub = stability.NewHub()
	a.apiAddress, a.groupRoot, a.groupPattern = cfg.API, cfg.Root, cfg.Pattern
	a.reset(airports[0])
	return a, nil
}

// ActivitySink is the single input for automatic sleep monitoring.  The
// application event loop remains the owner of sleeping and probe scheduling.
func (a *App) ActivitySink() chan<- activity.Event { return a.activityEvents }

// Log exposes the app's shared structured logger to background adapters.
// Adapters can add lifecycle records without owning a second log format.
func (a *App) Log(level events.Level, module, event, message string, fields map[string]any) {
	a.events.Log(level, module, event, message, fields)
}

func (a *App) reset(airport domain.Airport) {
	if a.stabilityHub != nil {
		a.stabilityHub.Reset()
	}
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
	a.sleepController = sleep.New(sleep.Config{Enabled: a.cfg.SleepEnabled, IdleAfter: a.cfg.SleepIdleAfter, QuietBytes: a.cfg.SleepQuietBytes, WakeWindow: a.cfg.SleepWakeWindow, WakeBytes: a.cfg.SleepWakeBytes}, now)
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
	a.confirmation.Clear()
	a.events.Record("startup", fmt.Sprintf("%s：开始约 %.0f 秒启动检查，期间不选择节点", airport.Name, a.cfg.Startup.Seconds()))
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
func (a *App) NodeStability(ctx context.Context, id string, since time.Time) (domain.NodeStability, error) {
	r, e := a.request(ctx, request{action: "node-stability", target: id, since: since})
	return r.stability, e
}
func (a *App) NodeLongTerm(ctx context.Context, id string) (domain.LongTermStability, error) {
	r, e := a.request(ctx, request{action: "node-longterm", target: id})
	return r.longTerm, e
}
func (a *App) SubscribeStability(id string) (<-chan domain.StabilityUpdate, func()) {
	return a.stabilityHub.Subscribe(id)
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
	now := time.Now()
	nodes := a.store.Views(a.current, a.phase, a.cfg.DelayGreen, a.cfg.DelayYellow)
	for i := range nodes {
		n := &nodes[i]
		switch {
		case a.phase == "startup":
			n.Frequency = "连续检查"
		case a.phase == "unavailable":
			n.Frequency = a.cfg.Recovery.String() + "重试"
		case n.Current:
			n.Frequency = a.cfg.Current.String()
		case n.RecoveryRemaining > 0:
			n.Frequency = "恢复复查"
		case n.Tier == domain.TierCandidate:
			n.Frequency = a.cfg.Candidate.String()
		default:
			n.Frequency = a.cfg.Ordinary.String()
		}
	}
	return domain.Snapshot{Settings: a.cfg.Preferences(), Revision: a.revision, Paused: a.paused, Sleeping: a.sleeping, SleepEnabled: a.sleepController.Enabled(), LastActivity: lastActivity, APIAddress: a.apiAddress, GroupRoot: a.groupRoot, GroupPattern: a.groupPattern, Mode: a.cfg.Mode, SelectionMode: a.selectionMode, RootSelection: a.rootSelection, ControlActive: a.controlActive, Phase: a.phase, AirportID: a.airport.ID, Airports: a.airports, CurrentID: a.current, PendingID: a.pending, APIHealthy: a.healthy, StartedAt: a.started, Now: now, StartupSeconds: a.cfg.Startup.Seconds(), ObservationSeconds: a.observeWindow.Duration().Seconds(), Nodes: nodes, Events: a.events.RecentSnapshot(), SleepRecords: a.sleepRecords.Snapshot(now)}
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
