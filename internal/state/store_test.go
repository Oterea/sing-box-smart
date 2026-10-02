package state

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func TestStoreRecordsBoundedHistoryAndViewsFailures(t *testing.T) {
	now := time.Now()
	s := New([]domain.Node{{ID: "a", Name: "A"}, {ID: "b", Name: "B"}}, 2)
	s.Record("a", domain.Probe{Success: true, DelayMS: 100, At: now})
	s.Record("a", domain.Probe{Success: true, DelayMS: 120, At: now.Add(time.Second)})
	s.Record("a", domain.Probe{Success: false, Error: "timeout", At: now.Add(2 * time.Second)})
	s.Record("b", domain.Probe{Success: true, DelayMS: 80, At: now})
	views := s.Views("a", "normal")
	if len(views) != 2 || len(views[0].Samples) != 2 || views[0].DelayMS != nil || !views[0].Current || views[0].Frequency != "3 秒" {
		t.Fatalf("unexpected current view: %+v", views[0])
	}
	if views[1].Score == nil || views[1].Checks != 1 || views[1].Frequency != "5 分钟" {
		t.Fatalf("unexpected empty view: %+v", views[1])
	}
	if !s.AllChecked() || !s.AnyAvailable() {
		t.Fatal("availability/check state wrong")
	}
}

func TestStoreClassifiesAndReschedulesByTier(t *testing.T) {
	s := New([]domain.Node{{ID: "a"}, {ID: "b"}, {ID: "c"}, {ID: "d"}}, 5)
	for _, x := range []struct {
		id    string
		delay float64
	}{{"a", 100}, {"b", 110}, {"c", 125}, {"d", 170}} {
		s.Record(x.id, domain.Probe{Success: true, DelayMS: x.delay, At: time.Now()})
	}
	s.Reclassify("a")
	if s.ByID["a"].Tier != domain.TierCurrent || s.ByID["b"].Tier != domain.TierCandidate || s.ByID["c"].Tier != domain.TierCandidate || s.ByID["d"].Tier != domain.TierOrdinary {
		t.Fatalf("tiers: a=%s b=%s c=%s", s.ByID["a"].Tier, s.ByID["b"].Tier, s.ByID["c"].Tier)
	}
	now := time.Now()
	s.Reschedule(now, "a", 3*time.Second, 30*time.Second, 5*time.Minute)
	if !s.ByID["a"].Plan.NormalDue.Equal(now.Add(3*time.Second)) || !s.ByID["b"].Plan.NormalDue.Equal(now) || s.ByID["c"].Plan.NormalDue.Before(now) {
		t.Fatalf("bad schedule: %+v %+v %+v", s.ByID["a"].Plan, s.ByID["b"].Plan, s.ByID["c"].Plan)
	}
	if !s.NearCandidate("d", .4) {
		t.Fatal("ordinary node near boundary was not detected")
	}
}

func TestViewsShowRecoveryAndStartupOverrides(t *testing.T) {
	s := New([]domain.Node{{ID: "a"}}, 3)
	n := s.ByID["a"]
	n.Tier = domain.TierCandidate
	n.Plan.RecoveryStep = 2
	if got := s.Views("other", "normal")[0].Frequency; got != "恢复复查" {
		t.Fatalf("frequency=%q", got)
	}
	if got := s.Views("other", "startup")[0].Frequency; got != "连续检查" {
		t.Fatalf("startup frequency=%q", got)
	}
	if got := s.Views("other", "unavailable")[0].Frequency; got != "3 秒重试" {
		t.Fatalf("unavailable frequency=%q", got)
	}
}
