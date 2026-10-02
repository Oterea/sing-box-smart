package httpapi

import (
	"reflect"
	"sing-box-smart/internal/domain"
	"sort"
)

type ssePayload struct {
	Full    bool     `json:"full"`
	Removed []string `json:"removed,omitempty"`
	domain.Snapshot
}

func makeSSEPayload(previous *domain.Snapshot, snapshot domain.Snapshot) ssePayload {
	payload := ssePayload{Full: previous == nil || previous.AirportID != snapshot.AirportID || previous.StartedAt != snapshot.StartedAt || previous.InstanceID != snapshot.InstanceID, Snapshot: snapshot}
	if payload.Full {
		return payload
	}
	changed := make([]domain.NodeView, 0)
	old := make(map[string]domain.NodeView, len(previous.Nodes))
	currentIDs := make(map[string]struct{}, len(snapshot.Nodes))
	for _, node := range previous.Nodes {
		old[node.ID] = node
	}
	for _, node := range snapshot.Nodes {
		currentIDs[node.ID] = struct{}{}
		if prior, ok := old[node.ID]; !ok || !reflect.DeepEqual(prior, node) {
			changed = append(changed, node)
		}
	}
	for id := range old {
		if _, ok := currentIDs[id]; !ok {
			payload.Removed = append(payload.Removed, id)
		}
	}
	sort.Strings(payload.Removed)
	payload.Nodes = changed
	return payload
}
