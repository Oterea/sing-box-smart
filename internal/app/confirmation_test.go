package app

import (
	"context"
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestNodeSwitchWaitsForThreeConfirmationResults(t *testing.T) {
	a := newFlowApp(t)
	defer a.probeCancel()
	a.current = a.store.Nodes[0].Info.ID
	a.selectionInitialized = true
	a.controlActive = true
	a.selectionMode = domain.SelectionManual
	target := a.store.Nodes[1]
	a.startNodeConfirmation(context.Background(), target, true)

	for i, success := range []bool{true, true, false} {
		target.Plan.InFlight = false
		target.Plan.Reason = "confirm"
		a.observe(context.Background(), observation{
			id:         target.Info.ID,
			generation: a.generation,
			probe:      domain.Probe{Success: success, DelayMS: 120, At: time.Now()},
		})
		if i < 2 && a.pending == "" {
			t.Fatalf("confirmation ended after result %d", i+1)
		}
	}
	if target.Checks != 3 {
		t.Fatalf("checks=%d, want 3 confirmation results", target.Checks)
	}
	if a.pending != "" || a.confirmation.Active() || a.switchBusy {
		t.Fatalf("failed confirmation did not clear state: pending=%q confirmation=%+v busy=%v", a.pending, a.confirmation, a.switchBusy)
	}
}

func TestConfirmationDeadlineCancelsPendingSwitch(t *testing.T) {
	a := newFlowApp(t)
	target := a.store.Nodes[1]
	a.pending = target.Info.ID
	a.pendingKind = "node"
	a.confirmation.Start(target.Info.ID, time.Now().Add(-time.Minute))
	a.tick(context.Background(), time.Now())
	if a.pending != "" || a.confirmation.Active() {
		t.Fatalf("expired confirmation remained active: pending=%q confirmation=%+v", a.pending, a.confirmation)
	}
	target.Plan.Reason = "confirm"
	a.observe(context.Background(), observation{
		id:         target.Info.ID,
		generation: a.generation,
		probe:      domain.Probe{Success: true, DelayMS: 120, At: time.Now()},
	})
	if a.pending != "" {
		t.Fatal("late confirmation result started a new switch")
	}
}
