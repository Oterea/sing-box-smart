package demo

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestDemoCatalogAndSelectionBoundaries(t *testing.T) {
	c := New()
	airports, err := c.Airports(context.Background())
	if err != nil || len(airports) != 3 || len(airports[0].Nodes) != 12 {
		t.Fatalf("catalog=%d err=%v", len(airports), err)
	}
	if got, err := c.Current(context.Background(), "proxy"); err != nil || got != airports[0].Selector {
		t.Fatalf("proxy current=%q err=%v", got, err)
	}
	if err := c.Select(context.Background(), airports[0].Selector, airports[0].Nodes[1].ID); err != nil {
		t.Fatal(err)
	}
	if got, _ := c.Current(context.Background(), airports[0].Selector); got != airports[0].Nodes[1].ID {
		t.Fatalf("node was not selected: %q", got)
	}
	if err := c.Select(context.Background(), airports[0].Selector, "missing"); err == nil {
		t.Fatal("accepted node from another selector")
	}
	if _, err := c.Current(context.Background(), "missing"); err == nil {
		t.Fatal("accepted unknown selector")
	}
}

func TestDemoProbeHonorsCancellationAndUnknownNodes(t *testing.T) {
	c := New()
	if _, err := c.Probe(context.Background(), "missing"); err == nil {
		t.Fatal("unknown node accepted")
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	if _, err := c.Probe(ctx, c.airports[0].Nodes[0].ID); !errors.Is(err, context.Canceled) {
		t.Fatalf("cancel error=%v", err)
	}
	ctx, cancel = context.WithTimeout(context.Background(), time.Millisecond)
	defer cancel()
	if _, err := c.Probe(ctx, c.airports[0].Nodes[11].ID); !errors.Is(err, context.DeadlineExceeded) {
		t.Fatalf("timeout error=%v", err)
	}
}
