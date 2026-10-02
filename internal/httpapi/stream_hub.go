package httpapi

import (
	"context"
	"sing-box-smart/internal/domain"
	"sync"
	"time"
)

const (
	snapshotInterval = 250 * time.Millisecond
	sseHeartbeat     = 15 * time.Second
)

// snapshotHub samples the app once for all connected SSE clients. A subscriber
// keeps only the newest snapshot: intermediate revisions are safe to skip
// because each client computes its delta from the last snapshot it received.
type snapshotHub struct {
	service Service

	mu          sync.Mutex
	nextID      int
	subscribers map[int]chan domain.Snapshot
	current     *domain.Snapshot
	cancel      context.CancelFunc
	instance    string
	lastRead    time.Time
}

func newSnapshotHub(service Service) *snapshotHub {
	return &snapshotHub{service: service, subscribers: make(map[int]chan domain.Snapshot)}
}

func (h *snapshotHub) subscribe() (<-chan domain.Snapshot, func()) {
	ch := make(chan domain.Snapshot, 1)
	h.mu.Lock()
	id := h.nextID
	h.nextID++
	h.subscribers[id] = ch
	if h.current != nil {
		ch <- *h.current
	}
	if len(h.subscribers) == 1 {
		ctx, cancel := context.WithCancel(context.Background())
		h.cancel = cancel
		go h.run(ctx)
	}
	h.mu.Unlock()
	return ch, func() {
		h.mu.Lock()
		if existing, ok := h.subscribers[id]; ok {
			delete(h.subscribers, id)
			close(existing)
		}
		if len(h.subscribers) == 0 && h.cancel != nil {
			h.cancel()
			h.cancel = nil
			h.current = nil
		}
		h.mu.Unlock()
	}
}

func (h *snapshotHub) run(ctx context.Context) {
	ticker := time.NewTicker(snapshotInterval)
	defer ticker.Stop()
	for {
		h.publish(ctx)
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func (h *snapshotHub) publish(parent context.Context) {
	ctx, cancel := context.WithTimeout(parent, 2*time.Second)
	snapshot, err := h.service.Snapshot(ctx)
	cancel()
	if err != nil {
		return
	}
	h.mu.Lock()
	defer h.mu.Unlock()
	if parent.Err() != nil || len(h.subscribers) == 0 {
		return
	}
	h.lastRead = time.Now()
	snapshot.InstanceID = h.instance
	if h.current != nil && h.current.Revision == snapshot.Revision {
		return
	}
	h.current = &snapshot
	for _, ch := range h.subscribers {
		select {
		case ch <- snapshot:
		default:
			// A slow browser does not block the app or other clients. The
			// next snapshot contains the latest complete state for this
			// subscriber's delta calculation.
			select {
			case <-ch:
			default:
			}
			ch <- snapshot
		}
	}
}

// fresh reports whether the app loop has responded recently, rather than
// treating an open TCP connection as evidence of a healthy backend.
func (h *snapshotHub) fresh() bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	return h.current != nil && time.Since(h.lastRead) < 3*time.Second
}
