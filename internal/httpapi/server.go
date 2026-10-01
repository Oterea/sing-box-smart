// Package httpapi exposes snapshots and sends commands to app; it contains no selection rules.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"reflect"
	"sing-box-smart/internal/domain"
	assets "sing-box-smart/web"
	"sync"
	"time"
)

type Service interface {
	Snapshot(context.Context) (domain.Snapshot, error)
	Control(context.Context, string, string) error
	ConfigureAPI(context.Context, string) error
	ConfigureFilters(context.Context, string, string) error
	ConfigureConnection(context.Context, string, string, string) error
	TestAPI(context.Context, string) error
}

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
	if len(h.subscribers) == 0 {
		return
	}
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

type ssePayload struct {
	Full    bool     `json:"full"`
	Removed []string `json:"removed,omitempty"`
	domain.Snapshot
}

func makeSSEPayload(previous *domain.Snapshot, snapshot domain.Snapshot) ssePayload {
	payload := ssePayload{Full: previous == nil || previous.AirportID != snapshot.AirportID, Snapshot: snapshot}
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
	payload.Nodes = changed
	return payload
}

func Handler(service Service) http.Handler {
	mux := http.NewServeMux()
	hub := newSnapshotHub(service)
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		s, err := service.Snapshot(ctx)
		if err != nil {
			write(w, 503, map[string]string{"error": "状态读取超时"})
			return
		}
		etag := fmt.Sprintf(`"%d"`, s.Revision)
		w.Header().Set("ETag", etag)
		if r.Header.Get("If-None-Match") == etag {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		write(w, 200, s)
	})
	mux.HandleFunc("GET /api/events", func(w http.ResponseWriter, r *http.Request) {
		flusher, ok := w.(http.Flusher)
		if !ok {
			write(w, 501, map[string]string{"error": "SSE 不受支持"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		ctx := r.Context()
		snapshots, unsubscribe := hub.subscribe()
		defer unsubscribe()
		var previous *domain.Snapshot
		heartbeat := time.NewTicker(sseHeartbeat)
		defer heartbeat.Stop()
		if _, err := fmt.Fprint(w, "retry: 3000\n\n"); err != nil {
			return
		}
		flusher.Flush()
		for {
			select {
			case <-ctx.Done():
				return
			case snapshot, ok := <-snapshots:
				if !ok {
					return
				}
				payload := makeSSEPayload(previous, snapshot)
				body, err := json.Marshal(payload)
				if err != nil || writeSSE(w, flusher, snapshot.Revision, body) != nil {
					return
				}
				copy := snapshot
				previous = &copy
			case <-heartbeat.C:
				if _, err := fmt.Fprint(w, ": keep-alive\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
	mux.HandleFunc("POST /api/config/filters", func(w http.ResponseWriter, r *http.Request) {
		var b struct {
			Root    string `json:"root"`
			Pattern string `json:"pattern"`
		}
		if err := decodeJSON(w, r, &b, 2048); err != nil {
			write(w, 400, map[string]string{"error": "请求无效"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		if err := service.ConfigureFilters(ctx, b.Root, b.Pattern); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 202, map[string]string{"status": "reloaded"})
	})
	mux.HandleFunc("POST /api/config/api/test", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			API string `json:"api"`
		}
		if err := decodeJSON(w, r, &body, 2048); err != nil {
			write(w, 400, map[string]string{"error": "请求无效"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := service.TestAPI(ctx, body.API); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, map[string]string{"status": "ok"})
	})
	mux.HandleFunc("POST /api/config/api", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			API string `json:"api"`
		}
		if err := decodeJSON(w, r, &body, 2048); err != nil || body.API == "" {
			write(w, 400, map[string]string{"error": "需要 api 地址"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 6*time.Second)
		defer cancel()
		if err := service.ConfigureAPI(ctx, body.API); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 202, map[string]string{"status": "reloaded"})
	})
	mux.HandleFunc("POST /api/config", func(w http.ResponseWriter, r *http.Request) {
		var body struct {
			API     string `json:"api"`
			Root    string `json:"root"`
			Pattern string `json:"pattern"`
		}
		if err := decodeJSON(w, r, &body, 4096); err != nil || body.API == "" {
			write(w, 400, map[string]string{"error": "需要 api、root 和 pattern"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 8*time.Second)
		defer cancel()
		if err := service.ConfigureConnection(ctx, body.API, body.Root, body.Pattern); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 202, map[string]string{"status": "reloaded"})
	})
	for _, action := range []string{"airport", "node", "recheck", "pause", "resume"} {
		action := action
		mux.HandleFunc("POST /api/control/"+action, func(w http.ResponseWriter, r *http.Request) {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || u.Scheme != "http" {
					write(w, 403, map[string]string{"error": "不允许跨站操作"})
					return
				}
			}
			var body struct {
				ID string `json:"id"`
			}
			r.Body = http.MaxBytesReader(w, r.Body, 4096)
			if err := decodeJSON(w, r, &body, 4096); err != nil {
				write(w, 400, map[string]string{"error": "请求需要 JSON 对象"})
				return
			}
			ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
			defer cancel()
			if err := service.Control(ctx, action, body.ID); err != nil {
				write(w, 409, map[string]string{"error": err.Error()})
				return
			}
			write(w, 202, map[string]string{"status": "accepted"})
		})
	}
	mux.Handle("GET /", http.FileServer(http.FS(assets.Files)))
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("Cache-Control", "no-store")
		w.Header().Set("Content-Security-Policy", "default-src 'self'; style-src 'self'; script-src 'self'; connect-src 'self'; frame-ancestors 'none'")
		if r.Method == "POST" {
			if origin := r.Header.Get("Origin"); origin != "" {
				u, err := url.Parse(origin)
				if err != nil || u.Host != r.Host || (u.Scheme != "http" && u.Scheme != "https") {
					write(w, 403, map[string]string{"error": "不允许跨站操作"})
					return
				}
			}
		}
		mux.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}

func decodeJSON(w http.ResponseWriter, r *http.Request, destination any, limit int64) error {
	decoder := json.NewDecoder(http.MaxBytesReader(w, r.Body, limit))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(destination); err != nil {
		return err
	}
	var extra any
	if err := decoder.Decode(&extra); err != io.EOF {
		if err == nil {
			return fmt.Errorf("multiple JSON values")
		}
		return err
	}
	return nil
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, revision uint64, body []byte) error {
	if _, err := fmt.Fprintf(w, "id: %d\nevent: state\ndata: %s\n\n", revision, body); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}
