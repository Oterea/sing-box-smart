package serviceconfig

import (
	"bytes"
	"encoding/json"
	"os"
	"path/filepath"
	"testing"
)

func TestDefaultReadAndAtomicValidatedApply(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "settings", "service.json")
	pending := filepath.Join(dir, "pending.json")
	var out bytes.Buffer
	if err := Command("read", path, pending, &out); err != nil {
		t.Fatal(err)
	}
	var s Settings
	if err := json.Unmarshal(out.Bytes(), &s); err != nil {
		t.Fatal(err)
	}
	if s.GRPCAddress != Defaults().GRPCAddress || s.GRPCSecret != "" {
		t.Fatalf("incorrect defaults %+v", s)
	}
	s.Listen = "127.0.0.1:9798"
	raw, _ := json.Marshal(s)
	if err := os.WriteFile(pending, raw, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Command("apply", path, pending, &out); err != nil {
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
		if err := Command("apply", path, pending, &out); err == nil {
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

func TestCanRemoveGRPCWhenRuntimeDisablesSleep(t *testing.T) {
	dir := t.TempDir()
	path := filepath.Join(dir, "service.json")
	pending := filepath.Join(dir, "pending")
	s := Defaults()
	s.GRPCAddress = ""
	b, _ := json.Marshal(s)
	if err := os.WriteFile(pending, b, 0600); err != nil {
		t.Fatal(err)
	}
	if err := Command("apply", path, pending, &bytes.Buffer{}); err != nil {
		t.Fatal(err)
	}
}
