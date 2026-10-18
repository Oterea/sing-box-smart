package app

import (
	"context"
	"net/url"
	"sing-box-smart/internal/grpcactivity"
	"time"
)

// Activity adapters are replaced only by the application loop. Cancellation
// completes before draining queued events, preventing old monitor data leaks.
func (a *App) stopActivity() {
	if a.activityCancel != nil {
		a.activityCancel()
		<-a.activityDone
		a.activityCancel = nil
	}
	for {
		select {
		case <-a.activityEvents:
		default:
			return
		}
	}
}
func (a *App) restartActivity(ctx context.Context) {
	a.stopActivity()
	if !a.cfg.SleepEnabled || a.cfg.GRPCAddress == "" {
		return
	}
	parsed, _ := url.Parse(a.cfg.TestURL)
	source := grpcactivity.New(grpcactivity.Config{Address: a.cfg.GRPCAddress, Secret: a.cfg.GRPCSecret, Interval: time.Second, ProbeDomains: []string{parsed.Hostname()}, ManagementPort: grpcactivity.PortFromListenAddress(a.cfg.Listen), Logger: a})
	sourceCtx, cancel := context.WithCancel(ctx)
	a.activityCancel = cancel
	a.activityDone = make(chan struct{})
	done := a.activityDone
	go func() { defer close(done); _ = source.Run(sourceCtx, a.ActivitySink()) }()
}
