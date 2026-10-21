package stability

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestLongTermAggregatesTimeBucketsAndReloads(t *testing.T) {
	path := t.TempDir() + "/stability.json"
	store, err := NewLongTerm(path)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 21, 9, 7, 0, 0, time.Local)
	for _, probe := range []domain.Probe{
		{At: base, Success: true, DelayMS: 100},
		{At: base.Add(2 * time.Minute), Success: true, DelayMS: 300},
		{At: base.Add(16 * time.Minute), Success: false, Error: "timeout"},
	} {
		if err := store.Record("airport", "node", "Node", probe); err != nil {
			t.Fatal(err)
		}
	}
	if err := store.Flush(); err != nil {
		t.Fatal(err)
	}
	data := store.Query("airport", "node", base.Add(time.Hour), 600)
	if len(data.Days) != 1 || len(data.Days[0].Buckets) != 2 {
		t.Fatalf("unexpected buckets: %+v", data)
	}
	if data.Days[0].Buckets[0].P50MS == nil || data.Days[0].Buckets[1].MaxConsecutiveFail != 1 {
		t.Fatalf("unexpected bucket metrics: %+v", data.Days[0].Buckets)
	}
	reloaded, err := NewLongTerm(path)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Query("airport", "node", base.Add(time.Hour), 600); len(got.Days) != 1 {
		t.Fatalf("reload lost data: %+v", got)
	}
}

func TestLongTermPrunesOlderThanFourteenDays(t *testing.T) {
	store, err := NewLongTerm(t.TempDir() + "/stability.json")
	if err != nil {
		t.Fatal(err)
	}
	old := time.Date(2026, 10, 1, 10, 0, 0, 0, time.Local)
	current := time.Date(2026, 10, 21, 10, 0, 0, 0, time.Local)
	if err := store.Record("airport", "node", "Node", domain.Probe{At: old, Success: true, DelayMS: 100}); err != nil {
		t.Fatal(err)
	}
	if err := store.Record("airport", "node", "Node", domain.Probe{At: current, Success: true, DelayMS: 100}); err != nil {
		t.Fatal(err)
	}
	data := store.Query("airport", "node", current, 600)
	if len(data.Days) != 1 || data.Days[0].Date != "2026-10-21" {
		t.Fatalf("old data was not pruned: %+v", data)
	}
}
