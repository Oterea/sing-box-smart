package httpapi

import (
	"context"
	"net/http"
	"sing-box-smart/internal/preferences"
	"time"
)

type settingsService interface {
	ConfigureSettings(context.Context, preferences.Settings) error
}

func registerSettings(mux *http.ServeMux, service Service) {
	mux.HandleFunc("GET /api/settings", func(w http.ResponseWriter, r *http.Request) {
		ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
		defer cancel()
		s, err := service.Snapshot(ctx)
		if err != nil {
			write(w, 503, map[string]string{"error": "设置读取失败"})
			return
		}
		write(w, 200, s.Settings)
	})
	mux.HandleFunc("POST /api/settings", func(w http.ResponseWriter, r *http.Request) {
		target, ok := service.(settingsService)
		if !ok {
			write(w, 501, map[string]string{"error": "不支持运行设置"})
			return
		}
		var body preferences.Settings
		if err := decodeJSON(w, r, &body, 8192); err != nil {
			write(w, 400, map[string]string{"error": "设置格式无效"})
			return
		}
		if err := body.Validate(); err != nil {
			write(w, 400, map[string]string{"error": err.Error()})
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		if err := target.ConfigureSettings(ctx, body); err != nil {
			write(w, 409, map[string]string{"error": err.Error()})
			return
		}
		write(w, 200, map[string]string{"status": "applied"})
	})
}
