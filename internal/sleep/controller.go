// Package sleep implements the automatic idle/sleep state machine.
// It has no knowledge of gRPC, Clash API, probes, or the application loop.
package sleep

import "time"

type Config struct {
	Enabled    bool
	IdleAfter  time.Duration
	QuietBytes int64
	WakeWindow time.Duration
	WakeBytes  int64
}

type State uint8

const (
	Active State = iota
	Sleeping
)

type Event struct {
	At            time.Time
	Probe         bool
	Active        bool
	UplinkDelta   int64
	DownlinkDelta int64
}

type Controller struct {
	cfg          Config
	state        State
	windowStart  time.Time
	lastActivity time.Time
	traffic      []trafficSample
}

type trafficSample struct {
	at    time.Time
	bytes int64
}

func New(cfg Config, now time.Time) *Controller {
	if cfg.IdleAfter <= 0 {
		cfg.IdleAfter = 10 * time.Minute
	}
	if cfg.QuietBytes <= 0 {
		cfg.QuietBytes = 64 * 1024
	}
	if cfg.WakeWindow <= 0 {
		cfg.WakeWindow = 5 * time.Second
	}
	if cfg.WakeBytes <= 0 {
		cfg.WakeBytes = 32 * 1024
	}
	return &Controller{cfg: cfg, windowStart: now, lastActivity: now}
}

func (c *Controller) Enabled() bool           { return c.cfg.Enabled }
func (c *Controller) State() State            { return c.state }
func (c *Controller) LastActivity() time.Time { return c.lastActivity }

// Observe records only non-probe traffic. A connection event without bytes is
// deliberately ignored: idle keep-alive connections must not keep smart
// awake, and the caller can wake quickly once a real traffic burst arrives.
func (c *Controller) Observe(e Event) bool {
	if !c.cfg.Enabled || e.Probe {
		return false
	}
	bytes := e.UplinkDelta + e.DownlinkDelta
	if bytes <= 0 {
		return false
	}
	when := e.At
	if when.IsZero() {
		when = time.Now()
	}
	c.traffic = append(c.traffic, trafficSample{at: when, bytes: bytes})
	c.prune(when)
	c.lastActivity = when
	if c.state == Sleeping && c.sumSince(when, c.cfg.WakeWindow) >= c.cfg.WakeBytes {
		c.state = Active
		return true
	}
	return false
}

// Tick returns true exactly once when the controller enters sleep.  The
// caller decides whether a pending switch makes sleeping unsafe.
func (c *Controller) Tick(now time.Time, blocked bool) bool {
	if !c.cfg.Enabled || c.state == Sleeping || blocked || c.windowStart.IsZero() {
		return false
	}
	c.prune(now)
	if now.Sub(c.windowStart) < c.cfg.IdleAfter {
		return false
	}
	if c.sumSince(now, c.cfg.IdleAfter) >= c.cfg.QuietBytes {
		return false
	}
	c.state = Sleeping
	return true
}

func (c *Controller) Wake() {
	c.WakeAt(time.Now())
}

func (c *Controller) WakeAt(now time.Time) {
	if c.cfg.Enabled {
		c.state = Active
		if now.IsZero() {
			now = time.Now()
		}
		c.windowStart = now
	}
}

func (c *Controller) prune(now time.Time) {
	keepAfter := now.Add(-c.cfg.IdleAfter)
	if c.cfg.WakeWindow > c.cfg.IdleAfter {
		keepAfter = now.Add(-c.cfg.WakeWindow)
	}
	first := 0
	for first < len(c.traffic) && c.traffic[first].at.Before(keepAfter) {
		first++
	}
	if first > 0 {
		c.traffic = append([]trafficSample(nil), c.traffic[first:]...)
	}
}

func (c *Controller) sumSince(now time.Time, window time.Duration) int64 {
	cutoff := now.Add(-window)
	var total int64
	for _, sample := range c.traffic {
		if !sample.at.Before(cutoff) && !sample.at.After(now) {
			total += sample.bytes
		}
	}
	return total
}
