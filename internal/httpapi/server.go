// Package httpapi exposes snapshots and sends commands to app; it contains no selection rules.
package httpapi

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"sing-box-smart/internal/domain"
	assets "sing-box-smart/web"
	"time"
)

type Service interface {
	Snapshot(context.Context) (domain.Snapshot, error)
	Control(context.Context, string, string) error
	ConfigureAPI(context.Context, string) error
	ConfigureFilters(context.Context, string, string) error
	TestAPI(context.Context, string) error
}

func Handler(service Service) http.Handler {
	mux := http.NewServeMux()
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
		ctx := r.Context()
		last := uint64(^uint64(0))
		ticker := time.NewTicker(250 * time.Millisecond)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
				s, err := service.Snapshot(ctx)
				if err != nil {
					return
				}
				if s.Revision == last {
					continue
				}
				body, err := json.Marshal(s)
				if err != nil {
					return
				}
				fmt.Fprintf(w, "event: state\\ndata: %s\\n\\n", body)
				flusher.Flush()
				last = s.Revision
			}
		}
	})
	mux.HandleFunc("POST /api/config/filters", func(w http.ResponseWriter, r *http.Request) {
		var b struct{ Root, Pattern string }
		if json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&b) != nil {
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
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&body); err != nil {
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
		if err := json.NewDecoder(http.MaxBytesReader(w, r.Body, 2048)).Decode(&body); err != nil || body.API == "" {
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
			dec := json.NewDecoder(r.Body)
			dec.DisallowUnknownFields()
			if err := dec.Decode(&body); err != nil {
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
