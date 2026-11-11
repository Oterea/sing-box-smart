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
