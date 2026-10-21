package stability

import (
	"sing-box-smart/internal/domain"
	"testing"
)

func TestHubPublishesNodeScopedUpdates(t *testing.T) {
	hub := NewHub()
	updates, closeSubscription := hub.Subscribe("node-a")
	defer closeSubscription()
	hub.Publish(domain.StabilityUpdate{NodeID: "node-b"})
	hub.Publish(domain.StabilityUpdate{NodeID: "node-a"})
	select {
	case update := <-updates:
		if update.NodeID != "node-a" || update.Sequence != 2 {
			t.Fatalf("unexpected update: %+v", update)
		}
	default:
		t.Fatal("node-scoped update was not delivered")
	}
}
