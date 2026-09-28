package events

import (
	"os"
	"path/filepath"
	"testing"
)

func TestLogFileDrainsBufferedWritesOnClose(t *testing.T) {
	dir := t.TempDir()
	w, err := Open(dir)
	if err != nil {
		t.Fatal(err)
	}
	const line = "probe\n"
	for i := 0; i < 1000; i++ {
		if _, err := w.Write([]byte(line)); err != nil {
			t.Fatal(err)
		}
	}
	if err := w.Close(); err != nil {
		t.Fatal(err)
	}
	b, err := os.ReadFile(filepath.Join(dir, "events.jsonl"))
	if err != nil {
		t.Fatal(err)
	}
	if len(b) != 1000*len(line) {
		t.Fatalf("got %d bytes", len(b))
	}
}
