package settings

import (
	"os"
	"path/filepath"
	"testing"
)

func TestConnectionPersistence(t *testing.T) {
	p := filepath.Join(t.TempDir(), "config", "connection.json")
	if e := Save(p, "http://127.0.0.1:9695"); e != nil {
		t.Fatal(e)
	}
	api, e := Load(p)
	if e != nil || api != "http://127.0.0.1:9695" {
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
