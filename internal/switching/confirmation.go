package switching

import "time"

// Confirmation owns the small state machine used before a node switch.
// The app event loop remains the owner of when probes are started; this type
// only tracks the bounded confirmation batch.
type Confirmation struct {
	Target    string
	Attempts  int
	Completed int
	StartedAt time.Time
	Deadline  time.Time
}

const (
	DefaultConfirmationAttempts = 3
	DefaultConfirmationLimit    = 15 * time.Second
)

func (c *Confirmation) Start(target string, now time.Time) {
	c.Target = target
	c.Attempts = DefaultConfirmationAttempts
	c.Completed = 0
	c.StartedAt = now
	c.Deadline = now.Add(DefaultConfirmationLimit)
}

func (c Confirmation) Active() bool { return c.Target != "" }

func (c Confirmation) Expired(now time.Time) bool {
	return c.Active() && !now.Before(c.Deadline)
}

func (c *Confirmation) Complete() bool {
	if !c.Active() {
		return false
	}
	c.Completed++
	return c.Completed >= c.Attempts
}

func (c *Confirmation) Clear() { *c = Confirmation{} }
