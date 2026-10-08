package switching

import (
	"context"
	"sing-box-smart/internal/gateway"
)

// ApplyGuarded checks the selection again after candidate confirmation. Clash
// API has no atomic compare-and-swap; this prevents overwriting changes observed
// before the write, but cannot lock an independent dashboard during PUT.
func ApplyGuarded(ctx context.Context, c gateway.Client, root, selector, current, target string, manual bool) Result {
	rootNow, err := c.Current(ctx, root)
	if err != nil {
		return Result{Err: err}
	}
	if rootNow != selector {
		return Result{RootActual: rootNow, Superseded: true}
	}
	nodeNow, err := c.Current(ctx, selector)
	if err != nil {
		return Result{Err: err}
	}
	if !manual && nodeNow != current {
		return Result{RootActual: rootNow, Actual: nodeNow, Superseded: true}
	}
	return Apply(ctx, c, selector, target)
}
