// Package observewindow tracks the common startup and resume observation
// window. It does not run probes; the app event loop remains the sole owner of
// probe scheduling and result application.
package observewindow

import "time"

type Window struct {
	duration time.Duration
	started  time.Time
	missing  map[string]struct{}
}

func (w *Window) Begin(duration time.Duration, ids []string, now time.Time) {
	w.duration = duration
	w.started = now
	w.missing = make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			w.missing[id] = struct{}{}
		}
	}
}

func (w *Window) Reset() {
	w.duration = 0
	w.started = time.Time{}
	w.missing = nil
}

func (w Window) Complete(id string) { delete(w.missing, id) }

func (w Window) Ready(now time.Time) bool {
	return !w.started.IsZero() && now.Sub(w.started) >= w.duration && len(w.missing) == 0
}

func (w Window) Duration() time.Duration { return w.duration }

func (w Window) Started() time.Time { return w.started }

func (w Window) Missing() int { return len(w.missing) }
