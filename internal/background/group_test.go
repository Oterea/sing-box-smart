package background

import (
	"sing-box-smart/internal/domain"
	"testing"
	"time"
)

func testAirport() domain.Airport {
	return domain.Airport{ID: "a", Name: "测试机场", Selector: "airport", Nodes: []domain.Node{
		{ID: "n1", Name: "节点 1"},
		{ID: "n2", Name: "节点 2"},
		{ID: "n3", Name: "节点 3"},
	}}
}

func TestScheduleSpreadsNodesAndCompletes(t *testing.T) {
	g := New(testAirport(), 4)
	now := time.Unix(100, 0)
	interval := 30 * time.Minute
	g.Schedule(now, interval)
	if got := len(g.DueNodes(now)); got != 1 {
		t.Fatalf("due at start = %d, want 1", got)
	}
	g.Complete("n1", now, interval)
	if got := len(g.DueNodes(now)); got != 0 {
		t.Fatalf("completed node became immediately due: %d", got)
	}
	if got := len(g.DueNodes(now.Add(10 * time.Minute))); got != 1 {
		t.Fatalf("staggered due at ten minutes = %d, want 1", got)
	}
}

func TestReclassifyUsesBackgroundCurrent(t *testing.T) {
	g := New(testAirport(), 4)
	g.CurrentID = "n2"
	g.Reclassify()
	views := g.Store.Views("n2", "background")
	if len(views) != 3 || !views[1].Current {
		t.Fatal("background current node was not classified as current")
	}
}

func TestUpdateAirportRefreshesMembershipAndPreservesHistory(t *testing.T) {
	g := New(testAirport(), 4)
	g.Store.Record("n1", domain.Probe{Success: true, DelayMS: 120, At: time.Unix(1, 0)})
	g.CurrentID = "n2"
	g.UpdateAirport(domain.Airport{ID: "a", Name: "测试机场", Selector: "airport", Nodes: []domain.Node{
		{ID: "n1", Name: "节点 1 新名称"},
		{ID: "n4", Name: "节点 4"},
	}}, 4)
	if len(g.Store.Nodes) != 2 || g.Store.ByID["n2"] != nil || g.Store.ByID["n4"] == nil {
		t.Fatalf("membership was not refreshed: %+v", g.Store.ByID)
	}
	if g.Store.ByID["n1"].Checks != 1 || g.Store.ByID["n1"].Info.Name != "节点 1 新名称" {
		t.Fatalf("existing node history was not preserved: %+v", g.Store.ByID["n1"])
	}
	if g.CurrentID != "" || len(g.Due) != 0 || len(g.InFlight) != 0 {
		t.Fatalf("stale current or schedule was retained: current=%q due=%v inflight=%v", g.CurrentID, g.Due, g.InFlight)
	}
}
