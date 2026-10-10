// Package sleep implements the automatic idle/sleep state machine.
// It has no knowledge of gRPC, Clash API, probes, or the application loop.
package sleep

import "time"

type Config struct {
	Enabled   bool
	IdleAfter time.Duration
}

type State uint8

const (
	Active State = iota
	Sleeping
)

type Event struct {
	At     time.Time
	Probe  bool
	Active bool
}

type Controller struct {
	cfg          Config
	state        State
	lastActivity time.Time
}

func New(cfg Config, now time.Time) *Controller {
	if cfg.IdleAfter <= 0 {
		cfg.IdleAfter = 10 * time.Minute
	}
	return &Controller{cfg: cfg, lastActivity: now}
}

func (c *Controller) Enabled() bool           { return c.cfg.Enabled }
func (c *Controller) State() State            { return c.state }
func (c *Controller) LastActivity() time.Time { return c.lastActivity }

// Observe records only real user activity.  Probe traffic is explicitly
// ignored so smart cannot keep itself awake.
func (c *Controller) Observe(e Event) bool {
	if !c.cfg.Enabled || e.Probe || !e.Active {
		return false
	}
	when := e.At
	if when.IsZero() {
		when = time.Now()
	}
	c.lastActivity = when
	if c.state == Sleeping {
		c.state = Active
		return true
	}
	return false
}

// Tick returns true exactly once when the controller enters sleep.  The
// caller decides whether a pending switch makes sleeping unsafe.
func (c *Controller) Tick(now time.Time, blocked bool) bool {
	if !c.cfg.Enabled || c.state == Sleeping || blocked || c.lastActivity.IsZero() {
		return false
	}
	if now.Sub(c.lastActivity) < c.cfg.IdleAfter {
		return false
	}
	c.state = Sleeping
	return true
}

func (c *Controller) Wake() {
	if c.cfg.Enabled {
		c.state = Active
	}
}
