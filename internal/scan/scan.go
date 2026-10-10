// Package scan tracks one bounded full refresh cycle.
//
// The app owns when a cycle starts and how its results are applied. This
// package only tracks the small piece of state needed to ensure that every
// node is attempted once before the cycle is released.
package scan

import "time"

type Kind string

const Resume Kind = "resume"

type Cycle struct {
	kind    Kind
	started time.Time
	pending map[string]struct{}
}

func (c *Cycle) Begin(kind Kind, ids []string, now time.Time) {
	c.kind = kind
	c.started = now
	c.pending = make(map[string]struct{}, len(ids))
	for _, id := range ids {
		if id != "" {
			c.pending[id] = struct{}{}
		}
	}
}

func (c *Cycle) Reset() {
	c.kind = ""
	c.started = time.Time{}
	c.pending = nil
}

func (c Cycle) Active() bool { return len(c.pending) > 0 }

func (c Cycle) Kind() Kind { return c.kind }

func (c Cycle) Started() time.Time { return c.started }

func (c Cycle) Pending(id string) bool {
	_, ok := c.pending[id]
	return ok
}

func (c *Cycle) Complete(id string) {
	if c.pending == nil {
		return
	}
	delete(c.pending, id)
}

func (c Cycle) Done() bool { return c.pending != nil && len(c.pending) == 0 }
