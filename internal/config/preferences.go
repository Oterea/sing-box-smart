package config

import (
	"path/filepath"
	"sing-box-smart/internal/preferences"
	"time"
)

func (c Config) Preferences() preferences.Settings {
	steps := make([]int, len(c.RecoverySteps))
	for i, v := range c.RecoverySteps {
		steps[i] = int(v / time.Second)
	}
	return preferences.Settings{NodeDisplayLimit: c.NodeDisplayLimit, TestURL: c.TestURL, Current: int(c.Current / time.Second), Candidate: int(c.Candidate / time.Second), Ordinary: int(c.Ordinary / time.Second), Timeout: int(c.Timeout / time.Second), Startup: int(c.Startup / time.Second), Resume: int(c.Resume / time.Second), Recovery: int(c.Recovery / time.Second), SleepEnabled: c.SleepEnabled, SleepIdle: int(c.SleepIdleAfter / time.Second), QuietBytes: c.SleepQuietBytes, WakeWindow: int(c.SleepWakeWindow / time.Second), WakeBytes: c.SleepWakeBytes, LogLevel: c.LogLevel, DelayGreen: c.DelayGreen, DelayYellow: c.DelayYellow, SwitchRatio: c.SwitchRatio, RecoveryDropRatio: c.RecoveryDropRatio, RecoveryDropMinMS: c.RecoveryDropMinMS, RecoveryGoodMaxMS: c.RecoveryGoodMaxMS, RecoverySteps: steps, HistoryLimit: c.HistoryLimit}
}
func (c *Config) ApplyPreferences(s preferences.Settings) {
	c.NodeDisplayLimit = s.NodeDisplayLimit
	c.TestURL = s.TestURL
	c.Current = time.Duration(s.Current) * time.Second
	c.Candidate = time.Duration(s.Candidate) * time.Second
	c.Ordinary = time.Duration(s.Ordinary) * time.Second
	c.Timeout = time.Duration(s.Timeout) * time.Second
	c.Startup = time.Duration(s.Startup) * time.Second
	c.Resume = time.Duration(s.Resume) * time.Second
	c.Recovery = time.Duration(s.Recovery) * time.Second
	c.SleepEnabled = s.SleepEnabled
	c.SleepIdleAfter = time.Duration(s.SleepIdle) * time.Second
	c.SleepQuietBytes = s.QuietBytes
	c.SleepWakeWindow = time.Duration(s.WakeWindow) * time.Second
	c.SleepWakeBytes = s.WakeBytes
	c.LogLevel = s.LogLevel
	c.DelayGreen = s.DelayGreen
	c.DelayYellow = s.DelayYellow
	c.SwitchRatio = s.SwitchRatio
	c.RecoveryDropRatio = s.RecoveryDropRatio
	c.RecoveryDropMinMS = s.RecoveryDropMinMS
	c.RecoveryGoodMaxMS = s.RecoveryGoodMaxMS
	c.HistoryLimit = s.HistoryLimit
	c.RecoverySteps = make([]time.Duration, len(s.RecoverySteps))
	for i, v := range s.RecoverySteps {
		c.RecoverySteps[i] = time.Duration(v) * time.Second
	}
}
func (c Config) PreferencesPath() string {
	if c.SettingsPath == "" {
		return ""
	}
	return filepath.Join(filepath.Dir(c.SettingsPath), "runtime.json")
}
