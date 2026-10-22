package preferences_test

import (
	"encoding/json"
	"math"
	"os"
	"path/filepath"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/preferences"
	"testing"
)

func TestRoundTripAndFutureDefaults(t *testing.T) {
	defaults := config.Default().Preferences()
	path := filepath.Join(t.TempDir(), "runtime.json")
	got, err := preferences.Load(path, defaults)
	if err != nil || got.WakeBytes != 1048576 {
		t.Fatalf("missing file: %+v %v", got, err)
	}
	defaults.NodeDisplayLimit = 12
	defaults.DelayGreen = 150
	defaults.LogLevel = "debug"
	if err = preferences.Save(path, defaults); err != nil {
		t.Fatal(err)
	}
	got, err = preferences.Load(path, config.Default().Preferences())
	if err != nil || got.NodeDisplayLimit != 12 || got.DelayGreen != 150 || got.LogLevel != "debug" {
		t.Fatalf("roundtrip: %+v %v", got, err)
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions %v", info.Mode())
	}
	if err = os.WriteFile(path, []byte(`{"delay_green_ms":100}`), 0600); err != nil {
		t.Fatal(err)
	}
	got, err = preferences.Load(path, config.Default().Preferences())
	if err != nil || got.NodeDisplayLimit != 30 || got.LongTermDisplayLimit != 5 || got.DelayGreen != 100 || got.WakeBytes != 1048576 {
		t.Fatalf("new defaults lost: %+v %v", got, err)
	}
}
func TestInvalidValuesCannotReplaceSavedSettings(t *testing.T) {
	cases := map[string]func(*preferences.Settings){
		"zero node limit":                 func(s *preferences.Settings) { s.NodeDisplayLimit = 0 },
		"excessive node limit":            func(s *preferences.Settings) { s.NodeDisplayLimit = 1001 },
		"zero long-term group limit":      func(s *preferences.Settings) { s.LongTermDisplayLimit = 0 },
		"excessive long-term group limit": func(s *preferences.Settings) { s.LongTermDisplayLimit = 1001 },
		"reversed colors":                 func(s *preferences.Settings) { s.DelayGreen = s.DelayYellow },
		"zero timeout":                    func(s *preferences.Settings) { s.Timeout = 0 },
		"overflow duration":               func(s *preferences.Settings) { s.Ordinary = 1 << 60 },
		"empty steps":                     func(s *preferences.Settings) { s.RecoverySteps = nil },
		"negative step":                   func(s *preferences.Settings) { s.RecoverySteps = []int{-1} },
		"bad URL":                         func(s *preferences.Settings) { s.TestURL = "file:///etc/passwd" },
		"bad level":                       func(s *preferences.Settings) { s.LogLevel = "trace" },
		"NaN":                             func(s *preferences.Settings) { s.SwitchRatio = math.NaN() },
		"infinity":                        func(s *preferences.Settings) { s.RecoveryDropRatio = math.Inf(1) },
		"negative bytes":                  func(s *preferences.Settings) { s.WakeBytes = -1 },
	}
	path := filepath.Join(t.TempDir(), "runtime.json")
	base := config.Default().Preferences()
	if err := preferences.Save(path, base); err != nil {
		t.Fatal(err)
	}
	before, _ := os.ReadFile(path)
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			s := config.Default().Preferences()
			mutate(&s)
			if err := preferences.Save(path, s); err == nil {
				t.Fatal("accepted invalid settings")
			}
			after, _ := os.ReadFile(path)
			if string(before) != string(after) {
				t.Fatal("changed valid file")
			}
		})
	}
}
func TestConfigMappingPreservesAllFields(t *testing.T) {
	c := config.Default()
	s := c.Preferences()
	s.NodeDisplayLimit = 15
	s.Current = 7
	s.RecoverySteps = []int{2, 4}
	c.ApplyPreferences(s)
	b, _ := json.Marshal(s)
	got, _ := json.Marshal(c.Preferences())
	if string(b) != string(got) {
		t.Fatalf("mapping lost fields: %s", got)
	}
}
