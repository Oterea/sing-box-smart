package httpapi

import (
	"context"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"sing-box-smart/internal/domain"
)

type sequenceService struct {
	mu    sync.Mutex
	calls int
	done  chan struct{}
	once  sync.Once
}

func (s *sequenceService) Snapshot(context.Context) (domain.Snapshot, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	if call == 3 {
		s.once.Do(func() { close(s.done) })
	}
	base := domain.Snapshot{Revision: uint64(call)}
	switch call {
	case 1:
		base.AirportID = "airport-a"
		base.Nodes = []domain.NodeView{{ID: "a"}, {ID: "b"}}
	case 2:
		base.AirportID = "airport-a"
		base.Nodes = []domain.NodeView{{ID: "a"}}
	default:
		base.AirportID = "airport-b"
		base.Nodes = []domain.NodeView{{ID: "c"}}
	}
	return base, nil
}

func (s *sequenceService) Control(context.Context, string, string) error { return nil }
func (s *sequenceService) ConfigureAPI(context.Context, string) error    { return nil }
func (s *sequenceService) ConfigureFilters(context.Context, string, string) error {
	return nil
}
func (s *sequenceService) ConfigureConnection(context.Context, string, string, string) error {
	return nil
}
func (s *sequenceService) TestAPI(context.Context, string) error { return nil }

func TestEventsSendRemovedNodesAndFullSnapshotOnAirportChange(t *testing.T) {
	service := &sequenceService{done: make(chan struct{})}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	req := httptest.NewRequest("GET", "/api/events", nil).WithContext(ctx)
	rec := httptest.NewRecorder()
	finished := make(chan struct{})
	go func() {
		Handler(service).ServeHTTP(rec, req)
		close(finished)
	}()

	select {
	case <-service.done:
	case <-time.After(2 * time.Second):
		t.Fatal("SSE did not produce three snapshots")
	}
	cancel()
	select {
	case <-finished:
	case <-time.After(time.Second):
		t.Fatal("SSE handler did not stop after request cancellation")
	}
	body := rec.Body.String()
	if !strings.Contains(body, "retry: 3000") || !strings.Contains(body, "id: 1") {
		t.Fatalf("missing SSE reconnect metadata: %s", body)
	}
	if !strings.Contains(body, `"full":false`) || !strings.Contains(body, `"removed":["b"]`) {
		t.Fatalf("missing delta removal event: %s", body)
	}
	if !strings.Contains(body, `"full":true`) || !strings.Contains(body, `"airport_id":"airport-b"`) {
		t.Fatalf("missing full event after airport change: %s", body)
	}
}

func TestControlRejectsNonSingleJSONObject(t *testing.T) {
	service := &sequenceService{done: make(chan struct{})}
	for _, body := range []string{`{} {}`, `{"unexpected":true}`} {
		req := httptest.NewRequest("POST", "/api/control/pause", strings.NewReader(body))
		req.Header.Set("Content-Type", "application/json")
		rec := httptest.NewRecorder()
		Handler(service).ServeHTTP(rec, req)
		if rec.Code != 400 {
			t.Fatalf("body %q returned %d, want 400", body, rec.Code)
		}
	}
}
