package observewindow

import (
	"testing"
	"time"
)

func TestWindowRequiresDurationAndEveryNode(t *testing.T) {
	now := time.Now()
	var w Window
	w.Begin(5*time.Second, []string{"a", "b"}, now)
	w.Complete("a")
	if w.Ready(now.Add(5 * time.Second)) {
		t.Fatal("window ended before every node completed")
	}
	w.Complete("b")
	if !w.Ready(now.Add(5 * time.Second)) {
		t.Fatal("window did not end after duration and coverage")
	}
	if w.Missing() != 0 {
		t.Fatalf("missing=%d, want 0", w.Missing())
	}
}
