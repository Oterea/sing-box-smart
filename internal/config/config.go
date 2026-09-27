package config

import (
	"fmt"
	"net"
	"time"
)

type Config struct {
	Listen, LogDir                                 string
	Startup, Current, Candidate, Recovery, Timeout time.Duration
	RecoveryBudget                                 int
	SwitchRatio                                    float64
	HistoryLimit                                   int
}

func Default() Config {
	return Config{Listen: "127.0.0.1:8787", LogDir: "logs", Startup: 10 * time.Second, Current: 2 * time.Second, Candidate: 30 * time.Second, Recovery: 3 * time.Second, Timeout: 4 * time.Second, RecoveryBudget: 20, SwitchRatio: 1.4, HistoryLimit: 24}
}
func (c Config) Validate() error {
	host, _, err := net.SplitHostPort(c.Listen)
	if err != nil {
		return err
	}
	ip := net.ParseIP(host)
	if ip == nil || !ip.IsLoopback() {
		return fmt.Errorf("prototype listen address must be a loopback IP")
	}
	if c.Startup <= 0 || c.Current <= 0 || c.Candidate <= 0 || c.Recovery <= 0 || c.Timeout <= 0 || c.RecoveryBudget <= 0 || c.SwitchRatio <= 1 || c.HistoryLimit < 1 {
		return fmt.Errorf("invalid timing, ratio, or history settings")
	}
	return nil
}
