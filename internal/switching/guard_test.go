package switching

import (
	"context"
	"errors"
	"testing"
)

type guardedClient struct {
	fakeClient
	root   string
	writes int
}

func (c *guardedClient) Current(ctx context.Context, group string) (string, error) {
	if c.currentErr != nil {
		return "", c.currentErr
	}
	if group == "proxy" {
		return c.root, nil
	}
	return c.fakeClient.Current(ctx, group)
}
func (c *guardedClient) Select(ctx context.Context, group, target string) error {
	c.writes++
	return c.fakeClient.Select(ctx, group, target)
}
func TestGuardedWriteHonorsExternalSelection(t *testing.T) {
	for _, tc := range []struct {
		name, root, node          string
		manual, write, superseded bool
	}{
		{"automatic unchanged", "a SMART", "old", false, true, false},
		{"external node", "a SMART", "external", false, false, true},
		{"external root", "Other AUTO", "old", false, false, true},
		{"manual explicit target", "a SMART", "external", true, true, false},
		{"manual external root", "Other AUTO", "old", true, false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			c := &guardedClient{root: tc.root, fakeClient: fakeClient{selected: tc.node}}
			r := ApplyGuarded(context.Background(), c, "proxy", "a SMART", "old", "new", tc.manual)
			if (c.writes > 0) != tc.write || r.Superseded != tc.superseded || r.Verified != tc.write {
				t.Fatalf("writes=%d result=%+v", c.writes, r)
			}
		})
	}
}
func TestGuardedReadErrorPreventsWrite(t *testing.T) {
	c := &guardedClient{fakeClient: fakeClient{currentErr: errors.New("disconnected")}}
	r := ApplyGuarded(context.Background(), c, "proxy", "a SMART", "old", "new", false)
	if c.writes != 0 || r.Err == nil || r.Verified {
		t.Fatalf("writes=%d result=%+v", c.writes, r)
	}
}
