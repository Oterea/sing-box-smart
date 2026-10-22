package stability

import (
	"os"
	"sing-box-smart/internal/clock"
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestLongTermAggregatesTimeBucketsAndReloads(t *testing.T) {
	path := t.TempDir() + "/stability.json"
	calendar := clock.MustCalendar("Asia/Shanghai", 15)
	store, err := NewLongTerm(path, calendar)
	if err != nil {
		t.Fatal(err)
	}
	base := time.Date(2026, 10, 21, 9, 7, 0, 0, calendar.Location)
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
	reloaded, err := NewLongTerm(path, calendar)
	if err != nil {
		t.Fatal(err)
	}
	if got := reloaded.Query("airport", "node", base.Add(time.Hour), 600); len(got.Days) != 1 {
		t.Fatalf("reload lost data: %+v", got)
	}
}

func TestLongTermPrunesOlderThanFourteenDays(t *testing.T) {
	store, err := NewLongTerm(t.TempDir()+"/stability.json", clock.MustCalendar("Asia/Shanghai", 15))
	if err != nil {
		t.Fatal(err)
	}
	calendar := clock.MustCalendar("Asia/Shanghai", 15)
	old := time.Date(2026, 10, 1, 10, 0, 0, 0, calendar.Location)
	current := time.Date(2026, 10, 21, 10, 0, 0, 0, calendar.Location)
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

func TestLongTermUsesConfiguredCalendarAcrossServerTimezone(t *testing.T) {
	calendar := clock.MustCalendar("Asia/Shanghai", 15)
	store, err := NewLongTermWithPersistence(calendar, nil)
	if err != nil {
		t.Fatal(err)
	}
	// 22:45 UTC is 06:45 on the following day in the configured calendar.
	instant := time.Date(2026, 10, 21, 22, 45, 0, 0, time.UTC)
	if err := store.Record("airport", "node", "Node", domain.Probe{At: instant, Success: true, DelayMS: 100}); err != nil {
		t.Fatal(err)
	}
	data := store.Query("airport", "node", instant, 600)
	if data.Timezone != "Asia/Shanghai" || len(data.Days) != 1 || data.Days[0].Date != "2026-10-22" {
		t.Fatalf("unexpected calendar day: %+v", data)
	}
	if len(data.Days[0].Buckets) != 1 || data.Days[0].Buckets[0].MinuteOfDay != 405 {
		t.Fatalf("unexpected bucket: %+v", data.Days[0].Buckets)
	}
}

func TestLongTermDropsLegacyStorageVersion(t *testing.T) {
	path := t.TempDir() + "/stability.json"
	if err := os.WriteFile(path, []byte(`{"version":1,"nodes":{"old":{"days":{"2026-10-21":{"date":"2026-10-21","buckets":{}}}}}}`), 0600); err != nil {
		t.Fatal(err)
	}
	store, err := NewLongTerm(path, clock.MustCalendar("Asia/Shanghai", 15))
	if err != nil {
		t.Fatal(err)
	}
	if got := store.Query("airport", "node", time.Now(), 600); len(got.Days) != 0 {
		t.Fatalf("legacy data was reused: %+v", got)
	}
}
