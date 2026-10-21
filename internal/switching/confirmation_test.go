package switching

import (
	"testing"
	"time"
)

func TestConfirmationIsBoundedAndCompletesAfterThreeResults(t *testing.T) {
	now := time.Unix(100, 0)
	var c Confirmation
	c.Start("node-a", now)
	if c.Attempts != 3 || c.Completed != 0 || c.Target != "node-a" {
		t.Fatalf("unexpected start state: %+v", c)
	}
	if c.Expired(now.Add(DefaultConfirmationLimit - time.Nanosecond)) {
		t.Fatal("confirmation expired before its deadline")
	}
	if c.Complete() || c.Completed != 1 {
		t.Fatal("first result should not complete the batch")
	}
	if c.Complete() || c.Completed != 2 {
		t.Fatal("second result completed the batch")
	}
	if !c.Complete() || c.Completed != 3 {
		t.Fatal("third result did not complete the batch")
	}
	if !c.Expired(now.Add(DefaultConfirmationLimit)) {
		t.Fatal("confirmation did not expire at its deadline")
	}
}
