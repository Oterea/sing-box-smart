package config

import (
	"testing"
	"time"
)

func TestParsePins(t *testing.T) {
	pins, err := ParsePins("Pei=pei PIN, ToLink=tolink PIN")
	if err != nil || len(pins) != 2 || pins[0].Selector != "pei PIN" {
		t.Fatalf("pins=%+v err=%v", pins, err)
	}
}
func TestParsePinsRejectsDuplicate(t *testing.T) {
	if _, err := ParsePins("a=x,b=x"); err == nil {
		t.Fatal("duplicate selector accepted")
	}
}
func TestRealConfigAllowsDynamicDiscoveryWithoutPins(t *testing.T) {
	c := Default()
	c.Mode = "real"
	if err := c.Validate(); err != nil {
		t.Fatal(err)
	}
}

func TestDefaultSchedulingPolicy(t *testing.T) {
	c := Default()
	if c.Ordinary != 5*time.Minute || len(c.RecoverySteps) != 4 {
		t.Fatalf("unexpected policy: %+v", c)
	}
}
