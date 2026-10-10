package scan

import (
	"testing"
	"time"
)

func TestCycleTracksEachNodeOnce(t *testing.T) {
	var c Cycle
	now := time.Now()
	c.Begin(Resume, []string{"a", "b", "a", ""}, now)
	if !c.Active() || !c.Pending("a") || !c.Pending("b") || !c.Started().Equal(now) {
		t.Fatal("cycle did not initialize its pending set")
	}
	c.Complete("a")
	if !c.Active() || c.Pending("a") {
		t.Fatal("completing one node changed the wrong state")
	}
	c.Complete("b")
	if !c.Done() || c.Active() {
		t.Fatal("cycle did not finish after all nodes completed")
	}
	c.Reset()
	if c.Done() || c.Active() || c.Pending("a") {
		t.Fatal("reset retained cycle state")
	}
}
