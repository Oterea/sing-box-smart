package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config", "connection.json")
	if e := Save(p, Connection{API: "http://127.0.0.1:9695", Root: "proxy", Pattern: "PIN$"}); e != nil {
		t.Fatal(e)
	}
	api, e := Load(p)
	if e != nil || api.API != "http://127.0.0.1:9695" || api.Pattern != "PIN$" {
		t.Fatal(api, e)
	}
	b, e := os.ReadFile(p)
	if e != nil || len(b) == 0 {
		t.Fatal(e)
	}
}
func TestAPIValidation(t *testing.T) {
	for _, v := range []string{"ftp://router", "http://user:pass@router", "http://router?a=b", "http://router#x", "missing"} {
		if _, e := NormalizeAPI(v); e == nil {
			t.Fatalf("accepted %q", v)
		}
	}
}

func TestLoadDefaultsAndRejectsMalformedFiles(t *testing.T) {
	dir := t.TempDir()
	missing, err := Load(filepath.Join(dir, "missing.json"))
	if err != nil || missing != (Connection{}) {
		t.Fatalf("missing=%+v err=%v", missing, err)
	}
	path := filepath.Join(dir, "bad.json")
	if err := os.WriteFile(path, []byte(`{"api":"not-an-api"}`), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := Load(path); err == nil {
		t.Fatal("malformed connection accepted")
	}
}
