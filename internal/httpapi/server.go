// Package httpapi exposes snapshots and sends commands to app; it contains no selection rules.
package httpapi

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sing-box-smart/internal/domain"
	assets "sing-box-smart/web"
	"strings"
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

type stabilityService interface {
	NodeStability(context.Context, string, time.Time) (domain.NodeStability, error)
	NodeLongTerm(context.Context, string) (domain.LongTermStability, error)
	SubscribeStability(string) (<-chan domain.StabilityUpdate, func())
}

func Handler(service Service) http.Handler {
	mux := http.NewServeMux()
	registerSettings(mux, service)
	hub := newSnapshotHub(service)
	var nonce [16]byte
	if _, err := rand.Read(nonce[:]); err != nil {
		panic(err)
	}
	hub.instance = hex.EncodeToString(nonce[:])
	mux.HandleFunc("GET /api/state", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		s, err := service.Snapshot(ctx)
		if err != nil {
			write(w, 503, map[string]string{"error": "状态读取超时"})
			return
		}
		s.InstanceID = hub.instance
		etag := fmt.Sprintf(`W/"%s-%d"`, hub.instance, s.Revision)
		w.Header().Set("ETag", etag)
		if matchesETag(r.Header.Get("If-None-Match"), etag) {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		write(w, 200, s)
	})
	mux.HandleFunc("GET /api/node/{id}/stability", func(w http.ResponseWriter, r *http.Request) {
		service, ok := service.(stabilityService)
		if !ok {
			write(w, http.StatusNotImplemented, map[string]string{"error": "节点详情不可用"})
			return
		}
		var since time.Time
		if raw := r.URL.Query().Get("since"); raw != "" {
			parsed, err := time.Parse(time.RFC3339Nano, raw)
			if err != nil {
				write(w, http.StatusBadRequest, map[string]string{"error": "since 时间无效"})
				return
			}
			since = parsed
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		detail, err := service.NodeStability(ctx, r.PathValue("id"), since)
		if err != nil {
			write(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		write(w, http.StatusOK, detail)
	})
	mux.HandleFunc("GET /api/node/{id}/stability/events", func(w http.ResponseWriter, r *http.Request) {
		service, ok := service.(stabilityService)
		if !ok {
			write(w, http.StatusNotImplemented, map[string]string{"error": "节点详情推送不可用"})
			return
		}
		flusher, ok := w.(http.Flusher)
		if !ok {
			write(w, http.StatusNotImplemented, map[string]string{"error": "SSE 不受支持"})
			return
		}
		w.Header().Set("Content-Type", "text/event-stream")
		w.Header().Set("Cache-Control", "no-cache")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("X-Accel-Buffering", "no")
		updates, unsubscribe := service.SubscribeStability(r.PathValue("id"))
		defer unsubscribe()
		if _, err := writeStream(w, "retry: 3000\n\n"); err != nil {
			return
		}
		flusher.Flush()
		heartbeat := time.NewTicker(sseHeartbeat)
		defer heartbeat.Stop()
		for {
			select {
			case <-r.Context().Done():
				return
			case update, open := <-updates:
				if !open {
					return
				}
				body, err := json.Marshal(update)
				if err != nil || writeNamedSSE(w, flusher, "stability", update.Sequence, body) != nil {
					return
				}
			case <-heartbeat.C:
				if _, err := writeStream(w, "event: heartbeat\ndata: {}\n\n"); err != nil {
					return
				}
				flusher.Flush()
			}
		}
	})
	mux.HandleFunc("GET /api/node/{id}/stability/long-term", func(w http.ResponseWriter, r *http.Request) {
		service, ok := service.(stabilityService)
		if !ok {
			write(w, http.StatusNotImplemented, map[string]string{"error": "长期稳定性不可用"})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		data, err := service.NodeLongTerm(ctx, r.PathValue("id"))
		if err != nil {
			write(w, http.StatusNotFound, map[string]string{"error": err.Error()})
			return
		}
		write(w, http.StatusOK, data)
	})
	mux.HandleFunc("GET /api/events", streamHandler(hub))
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
		if err := decodeJSON(w, r, &body, 4096); err != nil || body.API == "" || body.Pattern == "" {
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
	for _, action := range []string{"airport", "node", "recheck", "pause", "resume", "auto", "manual"} {
		action := action
		mux.HandleFunc("POST /api/control/"+action, func(w http.ResponseWriter, r *http.Request) {
			var body struct {
				ID string `json:"id"`
			}
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

func matchesETag(header, current string) bool {
	for _, candidate := range strings.Split(header, ",") {
		candidate = strings.TrimSpace(candidate)
		if candidate == "*" || candidate == current || strings.TrimPrefix(candidate, "W/") == strings.TrimPrefix(current, "W/") {
			return true
		}
	}
	return false
}

func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
