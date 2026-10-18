package httpapi

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/preferences"
	"strings"
	"testing"
)

type preferenceService struct {
	sequenceService
	settings preferences.Settings
	calls    int
}

func (s *preferenceService) Snapshot(context.Context) (domain.Snapshot, error) {
	return domain.Snapshot{Settings: s.settings}, nil
}
func (s *preferenceService) ConfigureSettings(_ context.Context, p preferences.Settings) error {
	s.calls++
	s.settings = p
	return nil
}
func TestSettingsHTTPValidationAndOrigins(t *testing.T) {
	s := &preferenceService{settings: config.Default().Preferences()}
	handler := Handler(s)
	raw, _ := json.Marshal(s.settings)
	for _, tc := range []struct {
		body, origin string
		code         int
	}{{string(raw), "", 200}, {`{"delay_green_ms":900}`, "", 400}, {string(raw), "https://untrusted.example", 403}, {string(raw) + " {}", "", 400}, {`null`, "", 400}} {
		request := httptest.NewRequest("POST", "http://smart/api/settings", strings.NewReader(tc.body))
		if tc.origin != "" {
			request.Header.Set("Origin", tc.origin)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		if response.Code != tc.code {
			t.Fatalf("body %s: got %d: %s", tc.body, response.Code, response.Body.String())
		}
	}
	if s.calls != 1 {
		t.Fatalf("invalid requests applied %d times", s.calls)
	}
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, httptest.NewRequest("GET", "http://smart/api/settings", nil))
	if response.Code != 200 || !strings.Contains(response.Body.String(), `"delay_green_ms":200`) {
		t.Fatal("read settings failed")
	}
}
