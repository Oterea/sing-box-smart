package sleep

import (
	"testing"
	"time"
)

func TestControllerSleepsAndWakesOnUserActivity(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(Config{Enabled: true, IdleAfter: time.Minute, QuietBytes: 64 * 1024, WakeWindow: 5 * time.Second, WakeBytes: 32 * 1024}, now)
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
	if !c.Observe(Event{At: now.Add(3 * time.Minute), Active: true, DownlinkDelta: 32 * 1024}) {
		t.Fatal("user activity did not wake controller")
	}
	if c.State() != Active {
		t.Fatal("state is not active")
	}
}

func TestControllerToleratesSmallBackgroundTraffic(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(Config{Enabled: true, IdleAfter: 10 * time.Minute, QuietBytes: 64 * 1024, WakeWindow: 5 * time.Second, WakeBytes: 32 * 1024}, now)
	for i := 0; i < 10; i++ {
		if c.Observe(Event{At: now.Add(time.Duration(i) * time.Minute), DownlinkDelta: 4 * 1024}) {
			t.Fatal("small background traffic woke controller")
		}
	}
	if !c.Tick(now.Add(10*time.Minute), false) {
		t.Fatal("did not sleep after a quiet traffic window")
	}
	if c.Observe(Event{At: now.Add(10*time.Minute + time.Second), DownlinkDelta: 8 * 1024}) {
		t.Fatal("small background burst woke controller")
	}
	if !c.Observe(Event{At: now.Add(10*time.Minute + 2*time.Second), DownlinkDelta: 24 * 1024}) {
		t.Fatal("meaningful traffic did not wake controller")
	}
}

func TestControllerHighTrafficPreventsSleepAndProbeTrafficIsIgnored(t *testing.T) {
	now := time.Unix(100, 0)
	c := New(Config{Enabled: true, IdleAfter: time.Minute, QuietBytes: 64 * 1024, WakeWindow: 5 * time.Second, WakeBytes: 32 * 1024}, now)
	if c.Observe(Event{At: now.Add(30 * time.Second), Probe: true, DownlinkDelta: 128 * 1024}) {
		t.Fatal("probe traffic woke controller")
	}
	c.Observe(Event{At: now.Add(40 * time.Second), DownlinkDelta: 64 * 1024})
	if c.Tick(now.Add(time.Minute), false) {
		t.Fatal("high traffic did not keep controller active")
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
