package real

import (
	"context"
	"net/http"
	"net/http/httptest"
	"sing-box-smart/internal/config"
	"testing"
)

func TestClientUsesClashAPI(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		switch {
		case r.Method == "GET" && r.URL.Path == "/proxies/pei PIN":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"type":"Selector","now":"node-a","all":["node-a","node-b"]}`))
		case r.Method == "GET" && r.URL.Path == "/proxies/proxy":
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"type":"Selector","now":"pei PIN","all":["pei PIN"]}`))
		case r.Method == "GET" && r.URL.Path == "/proxies/node-a/delay":
			if r.URL.Query().Get("url") == "" {
				t.Error("missing test URL")
			}
			w.Header().Set("Content-Type", "application/json")
			_, _ = w.Write([]byte(`{"delay":123}`))
		case r.Method == "PUT" && r.URL.Path == "/proxies/pei PIN":
			w.WriteHeader(http.StatusNoContent)
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(config.Config{API: srv.URL, Root: "proxy", TestURL: "https://example.com", Pins: []config.Pin{{ID: "pei PIN", Name: "Pei", Selector: "pei PIN"}}})
	as, err := c.Airports(context.Background())
	if err != nil || len(as) != 1 || len(as[0].Nodes) != 2 {
		t.Fatalf("airports=%+v err=%v", as, err)
	}
	p, err := c.Probe(context.Background(), "node-a")
	if err != nil || !p.Success || p.DelayMS != 123 {
		t.Fatalf("probe=%+v err=%v", p, err)
	}
	if err := c.Select(context.Background(), "pei PIN", "node-a"); err != nil {
		t.Fatal(err)
	}
	if err := c.Health(context.Background()); err != nil {
		t.Fatal(err)
	}
}
func TestProbeTransportErrorIsAPIError(t *testing.T) {
	c := New(config.Config{API: "http://127.0.0.1:1", TestURL: "https://example.com"})
	if _, err := c.Probe(context.Background(), "x"); err == nil {
		t.Fatal("transport error classified as node failure")
	}
}

func TestDynamicDiscoveryFallsBackToPinsWhenCatalogHasNoMatchingSelector(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		switch r.URL.Path {
		case "/proxies":
			_, _ = w.Write([]byte(`{"proxies":{"proxy":{"type":"Selector","all":["other"]},"other":{"type":"Selector","all":["node"]}}}`))
		case "/proxies/fallback PIN":
			_, _ = w.Write([]byte(`{"type":"Selector","all":["node"]}`))
		default:
			http.NotFound(w, r)
		}
	}))
	defer srv.Close()
	c := New(config.Config{API: srv.URL, Root: "proxy", Pattern: "PIN$", Pins: []config.Pin{{ID: "fallback PIN", Name: "fallback", Selector: "fallback PIN"}}})
	airports, err := c.Airports(context.Background())
	if err != nil || len(airports) != 1 || airports[0].Selector != "fallback PIN" {
		t.Fatalf("airports=%+v err=%v", airports, err)
	}
}

func TestSetAPIRejectsCredentialsQueryAndFragment(t *testing.T) {
	c := New(config.Config{API: "http://127.0.0.1:1"})
	for _, raw := range []string{"ftp://router", "http://user:pass@router", "http://router?q=1", "http://router#fragment"} {
		if err := c.SetAPI(raw); err == nil {
			t.Fatalf("accepted %q", raw)
		}
	}
	if err := c.SetAPI("http://router:9090/"); err != nil || c.endpoint("/health") != "http://router:9090/health" {
		t.Fatalf("valid SetAPI failed: %v", err)
	}
}
