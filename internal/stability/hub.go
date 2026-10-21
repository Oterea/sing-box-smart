package stability

import (
	"sing-box-smart/internal/domain"
	"sync"
)

// Hub publishes one-node stability deltas to the detail dialog without
// coupling the HTTP stream to the app event loop or to snapshot polling.
type Hub struct {
	mu          sync.Mutex
	nextID      int
	sequence    uint64
	subscribers map[string]map[int]chan domain.StabilityUpdate
}

func NewHub() *Hub { return &Hub{subscribers: make(map[string]map[int]chan domain.StabilityUpdate)} }

func (h *Hub) Subscribe(nodeID string) (<-chan domain.StabilityUpdate, func()) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.subscribers[nodeID] == nil {
		h.subscribers[nodeID] = make(map[int]chan domain.StabilityUpdate)
	}
	id := h.nextID
	h.nextID++
	ch := make(chan domain.StabilityUpdate, 32)
	h.subscribers[nodeID][id] = ch
	return ch, func() { h.unsubscribe(nodeID, id) }
}

func (h *Hub) Publish(update domain.StabilityUpdate) {
	h.mu.Lock()
	h.sequence++
	update.Sequence = h.sequence
	for _, ch := range h.subscribers[update.NodeID] {
		select {
		case ch <- update:
		default:
			// A slow detail dialog can resync through the REST since cursor;
			// keep the app loop non-blocking and retain the newest update.
			select {
			case <-ch:
			default:
			}
			ch <- update
		}
	}
	h.mu.Unlock()
}

func (h *Hub) Reset() {
	h.mu.Lock()
	defer h.mu.Unlock()
	for nodeID, subscribers := range h.subscribers {
		for id, ch := range subscribers {
			close(ch)
			delete(subscribers, id)
		}
		delete(h.subscribers, nodeID)
	}
}

func (h *Hub) unsubscribe(nodeID string, id int) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if subscribers := h.subscribers[nodeID]; subscribers != nil {
		if ch, ok := subscribers[id]; ok {
			delete(subscribers, id)
			close(ch)
		}
		if len(subscribers) == 0 {
			delete(h.subscribers, nodeID)
		}
	}
}
