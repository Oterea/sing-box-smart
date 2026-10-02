package httpapi

import (
	"encoding/json"
	"fmt"
	"net/http"
	"sing-box-smart/internal/domain"
	"time"
)

func streamHandler(hub *snapshotHub) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
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
		if _, err := writeStream(w, "retry: 3000\n\n"); err != nil {
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
				if !hub.fresh() {
					return
				}
				if _, err := writeStream(w, "event: heartbeat\ndata: {}\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	}
}

func writeSSE(w http.ResponseWriter, flusher http.Flusher, revision uint64, body []byte) error {
	if _, err := writeStream(w, fmt.Sprintf("id: %d\nevent: state\ndata: %s\n\n", revision, body)); err != nil {
		return err
	}
	flusher.Flush()
	return nil
}

// Bound every socket write (including flush) so abandoned clients release
// their subscription instead of retaining a handler and snapshot forever.
func writeStream(w http.ResponseWriter, text string) (int, error) {
	_ = http.NewResponseController(w).SetWriteDeadline(time.Now().Add(5 * time.Second))
	return fmt.Fprint(w, text)
}
