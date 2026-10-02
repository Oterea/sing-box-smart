package schedule

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestPlanRecoveryStepsAndDueReasons(t *testing.T) {
	now := time.Unix(100, 0)
	p := Plan{}
	if p.Due(now, "normal", false) != "normal" {
		t.Fatal("zero plan should be due")
	}
	steps := []time.Duration{time.Second, 2 * time.Second}
	if !p.StartRecovery(now, steps) || p.StartRecovery(now, steps) {
		t.Fatal("recovery start guard failed")
	}
	if p.Due(now, "normal", false) != "" || p.Due(now.Add(time.Second), "normal", false) != "recovery" {
		t.Fatal("recovery due wrong")
	}
	p.FinishRecovery(now.Add(time.Second), true, steps, 5*time.Second)
	if p.RecoveryStep != 2 || p.Due(now.Add(2*time.Second), "normal", false) != "" {
		t.Fatal("second recovery step wrong")
	}
	if p.Due(now.Add(3*time.Second), "normal", false) != "recovery" {
		t.Fatal("second recovery was not due")
	}
	p.FinishRecovery(now.Add(3*time.Second), true, steps, 5*time.Second)
	if p.RecoveryStep != 0 || p.Due(now.Add(3*time.Second), "normal", false) != "" {
		t.Fatal("successful recovery did not return to normal due")
	}
	if DurationFor(domain.TierCandidate, "current", "node", time.Second, 2*time.Second, 3*time.Second) != 2*time.Second {
		t.Fatal("candidate interval wrong")
	}
}
