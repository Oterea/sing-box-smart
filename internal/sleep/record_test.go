package sleep

import (
	"testing"
	"time"
)

func TestRecorderKeepsOnlyLatestRecords(t *testing.T) {
	base := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	r := NewRecorder(3)
	for i := 0; i < 4; i++ {
		start := base.Add(time.Duration(i) * time.Hour)
		r.Start(start, "idle")
		r.End(start.Add(5*time.Minute), "connection")
	}
	records := r.Snapshot(base.Add(5 * time.Hour))
	if len(records) != 3 {
		t.Fatalf("got %d records, want 3", len(records))
	}
	if !records[0].StartedAt.Equal(base.Add(time.Hour)) || !records[2].StartedAt.Equal(base.Add(3*time.Hour)) {
		t.Fatalf("oldest records were not evicted: %+v", records)
	}
	if records[0].DurationSeconds != 300 || records[0].Active {
		t.Fatalf("unexpected completed record: %+v", records[0])
	}
}

func TestRecorderActiveDurationAndDuplicateTransitions(t *testing.T) {
	base := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	r := NewRecorder(3)
	r.Start(base, "idle")
	r.Start(base.Add(time.Minute), "idle")
	records := r.Snapshot(base.Add(2 * time.Minute))
	if len(records) != 1 || !records[0].Active || records[0].DurationSeconds != 120 {
		t.Fatalf("unexpected active record: %+v", records)
	}
	r.End(base.Add(3*time.Minute), "connection")
	r.End(base.Add(4*time.Minute), "connection")
	if records := r.Snapshot(base.Add(4 * time.Minute)); len(records) != 1 || records[0].DurationSeconds != 180 {
		t.Fatalf("unexpected closed record after duplicate transitions: %+v", records)
	}
}

func TestRecorderLimitIncludesActiveRecord(t *testing.T) {
	base := time.Date(2026, 10, 10, 0, 0, 0, 0, time.UTC)
	r := NewRecorder(3)
	for i := 0; i < 3; i++ {
		start := base.Add(time.Duration(i) * time.Hour)
		r.Start(start, "idle")
		r.End(start.Add(time.Minute), "connection")
	}
	r.Start(base.Add(3*time.Hour), "idle")
	records := r.Snapshot(base.Add(3*time.Hour + 30*time.Second))
	if len(records) != 3 || !records[2].Active {
		t.Fatalf("active record exceeded limit: %+v", records)
	}
	if !records[0].StartedAt.Equal(base.Add(time.Hour)) {
		t.Fatalf("oldest record was not evicted before active record: %+v", records)
	}
}
