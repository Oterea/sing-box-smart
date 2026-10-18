package serviceconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestLegacyReadAndAtomicValidatedApply(t *testing.T) {
	dir := t.TempDir()
	legacy := filepath.Join(dir, "old.conf")
	path := filepath.Join(dir, "settings", "service.json")
	pending := filepath.Join(dir, "pending.json")
	if err := os.WriteFile(legacy, []byte("LISTEN='0.0.0.0:9797'\nGRPC_ADDR='127.0.0.1:9091'\nGRPC_SECRET='literal$(never-execute)'\nLOG_DIR='/var/log/sing-box-smart'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	var out bytes.Buffer
	if err := Command("read", path, pending, legacy, &out); err != nil {
		t.Fatal(err)
	}
	var s Settings
	if err := json.Unmarshal(out.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.GRPCAddress != "127.0.0.1:9091" || s.GRPCSecret != "literal$(never-execute)" {
		t.Fatalf("incorrect legacy parse %+v", s)
	}
	s.Listen = "127.0.0.1:9798"
	raw, _ := json.Marshal(s)
	if err := os.WriteFile(pending, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Command("apply", path, pending, legacy, &out); err != nil {
		t.Fatal(err)
	}
	got, err := Load(path, Defaults())
	if err != nil || got.Listen != s.Listen {
		t.Fatal("not saved")
	}
	info, _ := os.Stat(path)
	if info.Mode().Perm() != 0600 {
		t.Fatalf("permissions: %v", info.Mode())
	}
	before, _ := os.ReadFile(path)
	for _, bad := range []string{`{"listen":"oops"}`, `null`, `{"listen":"0.0.0.0:9797","unexpected":true}`, string(raw) + " {}"} {
		if err := os.WriteFile(pending, []byte(bad), 0600); err != nil {
			t.Fatal(err)
		}
		if err := Command("apply", path, pending, legacy, &out); err == nil {
			t.Fatalf("accepted %s", bad)
		}
		after, _ := os.ReadFile(path)
		if !bytes.Equal(before, after) {
			t.Fatal("invalid request replaced config")
		}
	}
}
func TestValidation(t *testing.T) {
	for _, bad := range []Settings{{Listen: "0.0.0.0:0", LogDir: "/var/log/smart"}, {Listen: "0.0.0.0:9797", LogDir: "relative"}, {Listen: "0.0.0.0:9797", GRPCAddress: "bad host:9090", LogDir: "/tmp/log"}} {
		if bad.Validate() == nil {
			t.Fatalf("accepted %+v", bad)
		}
	}
}

func TestCannotRemoveGRPCWhileSleepEnabled(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.json")
	pending := filepath.Join(dir, "pending")
	legacy := filepath.Join(dir, "old.conf")
	if err := os.WriteFile(legacy, []byte("SLEEP_MONITOR='1'\n"), 0600); err != nil {
		t.Fatal(err)
	}
	s := Defaults()
	s.GRPCAddress = ""
	b, _ := json.Marshal(s)
	os.WriteFile(pending, b, 0600)
	if err := Command("apply", path, pending, legacy, &bytes.Buffer{}); err == nil {
		t.Fatal("removed transport with legacy sleep enabled")
	}
	os.WriteFile(filepath.Join(dir, "runtime.json"), []byte(`{"sleep_enabled":false}`), 0600)
	if err := Command("apply", path, pending, legacy, &bytes.Buffer{}); err != nil {
		t.Fatal("runtime disable should override legacy:", err)
	}
}
