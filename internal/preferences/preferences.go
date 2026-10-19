// Package preferences owns the portable runtime settings and their validation.
// Durations are expressed in seconds and traffic in bytes at the API boundary.
package preferences

import (
	"encoding/json"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strings"
)

type Settings struct {
	NodeDisplayLimit  int     `json:"node_display_limit"`
	TestURL           string  `json:"test_url"`
	Current           int     `json:"current_seconds"`
	Candidate         int     `json:"candidate_seconds"`
	Ordinary          int     `json:"ordinary_seconds"`
	Timeout           int     `json:"timeout_seconds"`
	Startup           int     `json:"startup_seconds"`
	Resume            int     `json:"resume_seconds"`
	Recovery          int     `json:"retry_seconds"`
	SleepEnabled      bool    `json:"sleep_enabled"`
	SleepIdle         int     `json:"sleep_idle_seconds"`
	QuietBytes        int64   `json:"quiet_bytes"`
	WakeWindow        int     `json:"wake_window_seconds"`
	WakeBytes         int64   `json:"wake_bytes"`
	LogLevel          string  `json:"log_level"`
	DelayGreen        int     `json:"delay_green_ms"`
	DelayYellow       int     `json:"delay_yellow_ms"`
	SwitchRatio       float64 `json:"switch_ratio"`
	RecoveryDropRatio float64 `json:"recovery_drop_ratio"`
	RecoveryDropMinMS float64 `json:"recovery_drop_min_ms"`
	RecoveryGoodMaxMS float64 `json:"recovery_good_max_ms"`
	RecoverySteps     []int   `json:"recovery_steps_seconds"`
	HistoryLimit      int     `json:"history_limit"`
}

func (s Settings) Validate() error {
	u, err := url.Parse(s.TestURL)
	if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Hostname() == "" || u.User != nil || u.Fragment != "" {
		return fmt.Errorf("探测地址必须是有效的 HTTP/HTTPS 地址")
	}
	for _, v := range []struct {
		name      string
		v, lo, hi int
	}{
		{"当前节点间隔", s.Current, 1, 86400}, {"候选节点间隔", s.Candidate, 1, 86400}, {"普通节点间隔", s.Ordinary, 1, 86400},
		{"探测超时", s.Timeout, 1, 120}, {"启动观察", s.Startup, 1, 300}, {"恢复观察", s.Resume, 1, 300}, {"故障重试", s.Recovery, 1, 3600},
		{"睡眠时长", s.SleepIdle, 60, 86400}, {"唤醒窗口", s.WakeWindow, 1, 60}, {"历史数量", s.HistoryLimit, 1, 200},
		{"默认显示节点数", s.NodeDisplayLimit, 1, 1000}, {"绿色延迟上限", s.DelayGreen, 1, 59999}, {"黄色延迟上限", s.DelayYellow, 2, 60000},
	} {
		if v.v < v.lo || v.v > v.hi {
			return fmt.Errorf("%s需在 %d–%d 之间", v.name, v.lo, v.hi)
		}
	}
	if s.DelayGreen >= s.DelayYellow {
		return fmt.Errorf("绿色延迟上限必须小于黄色延迟上限")
	}
	if s.QuietBytes < 1 || s.QuietBytes > 1<<30 || s.WakeBytes < 1 || s.WakeBytes > 1<<30 {
		return fmt.Errorf("流量阈值需在 1 B–1 GiB 之间")
	}
	if !(s.SwitchRatio > 1 && s.SwitchRatio <= 10) || !(s.RecoveryDropRatio > 0 && s.RecoveryDropRatio < 1) || !(s.RecoveryDropMinMS > 0 && s.RecoveryDropMinMS <= 60000) || !(s.RecoveryGoodMaxMS > 0 && s.RecoveryGoodMaxMS <= 60000) {
		return fmt.Errorf("切换或恢复判断阈值无效")
	}
	if len(s.RecoverySteps) < 1 || len(s.RecoverySteps) > 20 {
		return fmt.Errorf("复查间隔需填写 1–20 项")
	}
	for _, v := range s.RecoverySteps {
		if v < 1 || v > 86400 {
			return fmt.Errorf("复查间隔需在 1–86400 秒之间")
		}
	}
	switch s.LogLevel {
	case "debug", "info", "warn", "error":
	default:
		return fmt.Errorf("日志级别无效")
	}
	return nil
}

// Load overlays saved values onto startup defaults, allowing new fields in future releases.
func Load(path string, defaults Settings) (Settings, error) {
	b, err := os.ReadFile(path)
	if os.IsNotExist(err) {
		return defaults, nil
	}
	if err != nil {
		return defaults, err
	}
	if err = json.Unmarshal(b, &defaults); err != nil {
		return defaults, err
	}
	return defaults, defaults.Validate()
}

// Save commits a complete validated file by atomic rename. It never edits shell configuration.
func Save(path string, s Settings) error {
	if err := s.Validate(); err != nil {
		return err
	}
	if strings.TrimSpace(path) == "" {
		return fmt.Errorf("未配置运行设置文件")
	}
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".runtime-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = json.NewEncoder(f).Encode(s); err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
