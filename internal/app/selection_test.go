package app

import (
	"context"
	"io"
	"log"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/demo"
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestManualModeKeepsProbingButBlocksAutomaticEvaluation(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.phase = "normal"
	a.rootSelection = a.airport.Selector
	a.controlActive = true
	a.current = a.store.Nodes[0].Info.ID
	a.selectionMode = domain.SelectionManual
	a.evaluate(context.Background())
	if a.pending != "" {
		t.Fatalf("manual mode scheduled an automatic switch: %q", a.pending)
	}
	if err := a.control(context.Background(), "auto", ""); err != nil {
		t.Fatal(err)
	}
	if a.selectionMode != domain.SelectionAuto {
		t.Fatalf("mode=%q, want auto", a.selectionMode)
	}
}

func TestExternalNodeSelectionEntersManualMode(t *testing.T) {
	client := demo.New()
	a, err := New(context.Background(), config.Default(), client, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.phase = "normal"
	a.rootSelection = a.airport.Selector
	a.controlActive = true
	a.current = a.store.Nodes[0].Info.ID
	a.selectionInitialized = true
	if err := client.Select(context.Background(), a.airport.Selector, a.store.Nodes[1].Info.ID); err != nil {
		t.Fatal(err)
	}
	a.applySelectionObservation(selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: a.airport.Selector, node: a.store.Nodes[1].Info.ID})
	if a.current != a.store.Nodes[1].Info.ID {
		t.Fatalf("current=%q, want externally selected node", a.current)
	}
	if a.selectionMode != domain.SelectionManual {
		t.Fatalf("mode=%q, want manual", a.selectionMode)
	}
}

func TestExternalAirportSelectionRebuildsRuntimeState(t *testing.T) {
	client := demo.New()
	a, err := New(context.Background(), config.Default(), client, log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.rootSelection = a.airport.Selector
	other := a.airports[1]
	if err := client.Select(context.Background(), "proxy", other.Selector); err != nil {
		t.Fatal(err)
	}
	if err := client.Select(context.Background(), other.Selector, other.Nodes[2].ID); err != nil {
		t.Fatal(err)
	}
	a.applySelectionObservation(selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: other.Selector, node: other.Nodes[2].ID})
	if a.airport.ID != other.ID || a.current != other.Nodes[2].ID {
		t.Fatalf("airport=%q current=%q, want external airport and node", a.airport.ID, a.current)
	}
	if a.selectionMode != domain.SelectionManual || a.phase != "startup" {
		t.Fatalf("mode=%q phase=%q, want manual startup", a.selectionMode, a.phase)
	}
}

func TestInitialSelectionUsesActualAirportWithoutEnteringManual(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer a.probeCancel()
	other := a.airports[2]
	a.applySelectionObservation(selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: other.Selector, node: other.Nodes[3].ID})
	if a.selectionMode != domain.SelectionAuto || a.current != other.Nodes[3].ID || a.airport.ID != other.ID || !a.controlActive {
		t.Fatalf("incorrect startup selection: %+v", a.snapshot())
	}
}

func TestUnmanagedRootStopsWritesAndCanBeSelectedAgain(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	defer a.probeCancel()
	a.rootSelection = a.airport.Selector
	a.selectionInitialized = true
	a.controlActive = true
	a.phase = "normal"
	a.current = a.store.Nodes[0].Info.ID
	a.applySelectionObservation(selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: "Other AUTO"})
	a.evaluate(context.Background())
	if a.controlActive || a.pending != "" || a.selectionMode != domain.SelectionManual {
		t.Fatal("unmanaged root still controlled")
	}
	if err := a.control(context.Background(), "node", a.store.Nodes[1].Info.ID); err == nil {
		t.Fatal("node control accepted while root unmanaged")
	}
	// Selecting the same monitored group must still write the root, rather than
	// returning early merely because the local airport ID hasn't changed.
	if err := a.control(context.Background(), "airport", a.airport.ID); err != nil {
		t.Fatal(err)
	}
	select {
	case c := <-a.changes:
		a.switched(c)
	case <-time.After(time.Second):
		t.Fatal("root activation not executed")
	}
	if !a.controlActive || a.rootSelection != a.airport.Selector {
		t.Fatal("root activation not confirmed")
	}
}

func TestManualModeCancelsPendingAutomaticConfirmation(t *testing.T) {
	a := newFlowApp(t)
	defer a.probeCancel()
	a.pending = a.store.Nodes[1].Info.ID
	a.pendingKind = "node"
	if err := a.control(context.Background(), "manual", ""); err != nil {
		t.Fatal(err)
	}
	if a.pending != "" || a.selectionMode != domain.SelectionManual {
		t.Fatal("automatic confirmation not canceled")
	}
	n := a.store.Nodes[1]
	n.Plan.Reason = "confirm"
	a.observe(context.Background(), observation{id: n.Info.ID, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 50, At: time.Now()}})
	if n.Checks != 1 || a.switchBusy || a.pending != "" {
		t.Fatal("manual mode failed to score or started a write")
	}
}

func TestStaleSyncCannotOverwriteNewSelectionOrConfiguration(t *testing.T) {
	a := newFlowApp(t)
	defer a.probeCancel()
	a.pending = ""
	a.current = a.store.Nodes[0].Info.ID
	a.selectionInitialized = true
	a.controlActive = true
	a.rootSelection = a.airport.Selector
	stale := selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: a.airport.Selector, node: a.store.Nodes[1].Info.ID}
	a.selectionEpoch++
	a.applySelectionObservation(stale)
	if a.current != a.store.Nodes[0].Info.ID {
		t.Fatal("stale selection overwritten")
	}
	stale.epoch = a.selectionEpoch
	stale.generation--
	a.applySelectionObservation(stale)
	if a.current != a.store.Nodes[0].Info.ID {
		t.Fatal("stale configuration overwritten")
	}
}

func TestManualSelectionOfCurrentNodeLocksMode(t *testing.T) {
	a := newFlowApp(t)
	defer a.probeCancel()
	a.pending = ""
	a.controlActive = true
	a.selectionInitialized = true
	a.current = a.store.Nodes[0].Info.ID
	if err := a.control(context.Background(), "node", a.current); err != nil {
		t.Fatal(err)
	}
	if a.selectionMode != domain.SelectionManual || a.pending != "" {
		t.Fatal("current node click did not lock manual mode")
	}
}

func TestManualModeRecoversAvailabilityAndScoresFailures(t *testing.T) {
	a := newFlowApp(t)
	defer a.probeCancel()
	a.pending = ""
	a.selectionMode = domain.SelectionManual
	a.phase = "unavailable"
	n := a.store.Nodes[1]
	n.Plan.Reason = "normal"
	a.observe(context.Background(), observation{id: n.Info.ID, generation: a.generation, probe: domain.Probe{Success: true, DelayMS: 120, At: time.Now()}})
	if a.phase != "normal" || n.Checks != 1 || a.pending != "" {
		t.Fatal("manual availability recovery failed")
	}
	n.Plan.Reason = "normal"
	a.observe(context.Background(), observation{id: n.Info.ID, generation: a.generation, probe: domain.Probe{Success: false, Error: "timeout", At: time.Now()}})
	if n.Checks != 2 || len(n.History) != 2 || n.Last.Success || a.switchBusy {
		t.Fatal("manual failure did not score normally")
	}
}

func TestSyncDuringPauseUpdatesSelectionWithoutResumingDetection(t *testing.T) {
	a := newFlowApp(t)
	defer a.probeCancel()
	a.pending = ""
	a.pauseDetection()
	a.rootSelection = a.airport.Selector
	a.current = a.store.Nodes[0].Info.ID
	a.selectionInitialized = true
	next := a.store.Nodes[1].Info.ID
	a.applySelectionObservation(selectionObservation{generation: a.generation, epoch: a.selectionEpoch, root: a.airport.Selector, node: next})
	if !a.paused || a.current != next || a.selectionMode != domain.SelectionManual {
		t.Fatal("paused sync inconsistent")
	}
}
