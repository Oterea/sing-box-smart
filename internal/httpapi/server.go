// Package httpapi exposes snapshots and sends commands to app; it contains no selection rules.
package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"sing-box-smart/internal/domain"
	assets "sing-box-smart/web"
	"time"
)

type Service interface {
	Snapshot(context.Context) (domain.Snapshot, error)
	Control(context.Context, string, string) error
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
		write(w, 200, s)
	})
	for _, action := range []string{"airport", "node", "recheck"} {
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
		mux.ServeHTTP(w, r)
	})
}
func write(w http.ResponseWriter, status int, value any) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(value)
}
