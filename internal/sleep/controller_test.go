package sleep

import (
	"testing"
	"time"
)

func TestControllerSleepsAndWakesOnUserActivity(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(Config{Enabled: true, IdleAfter: time.Minute}, now)
	if c.Tick(now.Add(59*time.Second), false) {
		t.Fatal("slept too early")
	}
	if !c.Tick(now.Add(time.Minute), false) {
		t.Fatal("did not sleep")
	}
	if c.State() != Sleeping {
		t.Fatal("state is not sleeping")
	}
	if c.Observe(Event{At: now.Add(2 * time.Minute), Probe: true, Active: true}) {
		t.Fatal("probe woke controller")
	}
	if c.Observe(Event{At: now.Add(2 * time.Minute), Active: false}) {
		t.Fatal("closed idle connection woke controller")
	}
	if !c.Observe(Event{At: now.Add(3 * time.Minute), Active: true}) {
		t.Fatal("user activity did not wake controller")
	}
	if c.State() != Active {
		t.Fatal("state is not active")
	}
}

func TestControllerDoesNotSleepWhileBlocked(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(Config{Enabled: true, IdleAfter: time.Minute}, now)
	if c.Tick(now.Add(2*time.Minute), true) {
		t.Fatal("slept while blocked")
	}
	if c.State() != Active {
		t.Fatal("state changed while blocked")
	}
}
