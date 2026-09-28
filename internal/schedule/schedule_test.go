package schedule

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestRecoveryDueWinsOverClearedNormalDue(t *testing.T) {
	now := time.Now()
	p := Plan{}
	p.StartRecovery(now, []time.Duration{3 * time.Second, 6 * time.Second})
	if got := p.Due(now.Add(3*time.Second), "normal", false); got != "recovery" {
		t.Fatalf("got %q", got)
	}
	p.FinishRecovery(now.Add(3*time.Second), true, []time.Duration{3 * time.Second, 6 * time.Second}, 5*time.Minute)
	if p.RecoveryStep != 2 {
		t.Fatalf("step=%d", p.RecoveryStep)
	}
}

func TestDurationForTier(t *testing.T) {
	if got := DurationFor(domain.TierOrdinary, "", "x", 2*time.Second, 30*time.Second, 5*time.Minute); got != 5*time.Minute {
		t.Fatal(got)
	}
	if got := DurationFor(domain.TierCandidate, "", "x", 2*time.Second, 30*time.Second, 5*time.Minute); got != 30*time.Second {
		t.Fatal(got)
	}
	if got := DurationFor(domain.TierOrdinary, "x", "x", 2*time.Second, 30*time.Second, 5*time.Minute); got != 2*time.Second {
		t.Fatal(got)
	}
}
