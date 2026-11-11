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

func TestBackgroundScheduleResetClearsInFlightAfterLifecycleCancel(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	a.tickBackground(context.Background(), time.Now())
	for id, group := range a.backgroundGroups {
		if id == a.airport.ID || len(group.Store.Nodes) == 0 {
			continue
		}
		group.InFlight[group.Store.Nodes[0].Info.ID] = true
	}
	a.resetBackgroundSchedule()
	if a.backgroundScheduleInterval != 0 {
		t.Fatal("background interval was not reset")
	}
	for _, group := range a.backgroundGroups {
		if len(group.InFlight) != 0 || len(group.Due) != 0 {
			t.Fatalf("stale background schedule remained: due=%v in_flight=%v", group.Due, group.InFlight)
		}
	}
}

func TestBackgroundObservationRejectsStaleGenerationAndPublishesErrors(t *testing.T) {
	a, err := New(context.Background(), config.Default(), demo.New(), log.New(io.Discard, "", 0))
	if err != nil {
		t.Fatal(err)
	}
	var groupID string
	for id := range a.backgroundGroups {
		if id != a.airport.ID {
			groupID = id
			break
		}
	}
	if groupID == "" {
		t.Skip("demo client has no background airport")
	}
	group := a.backgroundGroups[groupID]
	nodeID := group.Store.Nodes[0].Info.ID
	before := group.Store.ByID[nodeID].Checks
	oldGeneration := a.generation
	a.generation++
	a.observeBackground(observation{groupID: groupID, id: nodeID, generation: oldGeneration, probe: domain.Probe{Success: true, DelayMS: 20, At: time.Now()}})
	if group.Store.ByID[nodeID].Checks != before {
		t.Fatal("stale background observation was recorded")
	}
	revision := a.revision
	a.observeBackground(observation{groupID: groupID, id: nodeID, generation: a.generation, err: context.DeadlineExceeded})
	if a.revision != revision+1 {
		t.Fatalf("background transport error did not publish state revision: got %d want %d", a.revision, revision+1)
	}
}
