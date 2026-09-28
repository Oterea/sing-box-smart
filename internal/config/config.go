package config

import (
	"fmt"
	"net"
	"strings"
	"time"
)

type Pin struct{ ID, Name, Selector string }
type Config struct {
	Mode, API, Root, TestURL, Listen, LogDir                 string
	Pins                                                     []Pin
	Startup, Current, Candidate, Ordinary, Recovery, Timeout time.Duration
	RecoverySteps                                            []time.Duration
	RecoveryDropRatio, RecoveryDropMinMS                     float64
	SwitchRatio                                              float64
	HistoryLimit                                             int
}

func Default() Config {
	return Config{Mode: "demo", API: "http://127.0.0.1:9695", Root: "proxy", TestURL: "https://www.gstatic.com/generate_204", Listen: "127.0.0.1:8787", LogDir: "logs", Startup: 10 * time.Second, Current: 2 * time.Second, Candidate: 30 * time.Second, Ordinary: 5 * time.Minute, Recovery: 3 * time.Second, Timeout: 4 * time.Second, RecoverySteps: []time.Duration{3 * time.Second, 6 * time.Second, 12 * time.Second, 20 * time.Second}, RecoveryDropRatio: .75, RecoveryDropMinMS: 100, SwitchRatio: 1.4, HistoryLimit: 24}
}
func ParsePins(raw string) ([]Pin, error) {
	var out []Pin
	seen := map[string]bool{}
	for _, part := range strings.Split(raw, ",") {
		part = strings.TrimSpace(part)
		if part == "" {
			continue
		}
		bits := strings.SplitN(part, "=", 2)
		if len(bits) != 2 {
			return nil, fmt.Errorf("pin must be name=selector: %q", part)
		}
		name, selector := strings.TrimSpace(bits[0]), strings.TrimSpace(bits[1])
		if name == "" || selector == "" || seen[selector] {
			return nil, fmt.Errorf("invalid or duplicate pin: %q", part)
		}
		seen[selector] = true
		out = append(out, Pin{ID: selector, Name: name, Selector: selector})
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("at least one pin is required")
	}
	return out, nil
}
func (c Config) Validate() error {
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || (ip.IsUnspecified() == false && !ip.IsLoopback() && !ip.Equal(net.ParseIP("192.168.7.1"))) {
		return fmt.Errorf("listen address must be loopback, unspecified, or an explicitly allowed LAN address")
	}
	if c.Mode != "demo" && c.Mode != "real" {
		return fmt.Errorf("mode must be demo or real")
	}
	if c.Mode == "real" && (c.API == "" || c.TestURL == "" || len(c.Pins) == 0) {
		return fmt.Errorf("real mode requires api, test-url, and pins")
	}
	if c.Startup <= 0 || c.Current <= 0 || c.Candidate <= 0 || c.Ordinary <= 0 || c.Recovery <= 0 || c.Timeout <= 0 || len(c.RecoverySteps) == 0 || c.RecoveryDropRatio <= 0 || c.RecoveryDropRatio >= 1 || c.RecoveryDropMinMS <= 0 || c.SwitchRatio <= 1 || c.HistoryLimit < 1 {
		return fmt.Errorf("invalid timing, ratio, or history settings")
	}
	return nil
}
