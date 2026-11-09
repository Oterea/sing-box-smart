package configbundle

import (
	"encoding/json"
	"os"
	"path/filepath"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/preferences"
	"sing-box-smart/internal/serviceconfig"
	"sing-box-smart/internal/settings"
	"testing"
)

func TestPackagedDefaultsAreValid(t *testing.T) {
	root := filepath.Join("..", "..", "deploy", "defaults")
	var connection settings.Connection
	readJSON(t, filepath.Join(root, "connection.json"), &connection)
	if _, err := settings.NormalizeAPI(connection.API); err != nil {
		t.Fatal(err)
	}
	if _, err := settings.NormalizePattern(connection.Pattern); err != nil {
		t.Fatal(err)
	}

	var runtime preferences.Settings
	readJSON(t, filepath.Join(root, "runtime.json"), &runtime)
	if err := runtime.Validate(); err != nil {
		t.Fatal(err)
	}

	var service serviceconfig.Settings
	readJSON(t, filepath.Join(root, "service.json"), &service)
	if err := service.Validate(); err != nil {
		t.Fatal(err)
	}
	if config.Default().Root != connection.Root {
		t.Fatalf("default root mismatch: %q", connection.Root)
	}
}

func TestRecoveryReadKeepsInvalidValuesEditable(t *testing.T) {
	path := filepath.Join(t.TempDir(), "connection.json")
	if err := os.WriteFile(path, []byte(`{"api":"not-an-api","root":"missing","pattern":"["}`), 0600); err != nil {
		t.Fatal(err)
	}
	c, err := loadConnectionForEdit(path, settings.Connection{API: "http://127.0.0.1:9695", Root: "proxy", Pattern: "PIN$"})
	if err != nil {
		t.Fatal(err)
	}
	if c.API != "not-an-api" || c.Root != "missing" || c.Pattern != "[" {
		t.Fatalf("values were not preserved: %+v", c)
	}
}

func readJSON(t *testing.T, path string, dst any) {
	t.Helper()
	b, err := os.ReadFile(path)
	if err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(b, dst); err != nil {
		t.Fatal(err)
	}
}
