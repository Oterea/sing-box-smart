// Package switching changes a selector and reads it back; it never changes scores.
package switching

import (
	"context"
	"fmt"
	"sing-box-smart/internal/gateway"
)

type Result struct {
	Actual     string
	Verified   bool
	Err        error
	RootActual string
	Superseded bool
}

func Apply(ctx context.Context, c gateway.Client, group, target string) Result {
	putErr := c.Select(ctx, group, target)
	// Read back even when PUT errors: the server might have applied the request.
	actual, err := c.Current(ctx, group)
	if err != nil {
		return Result{Err: fmt.Errorf("selection unconfirmed: %w", err)}
	}
	if actual == target {
		return Result{Actual: actual, Verified: true}
	}
	if putErr != nil {
		return Result{Actual: actual, Err: putErr}
	}
	return Result{Actual: actual, Err: fmt.Errorf("selector readback differs from requested target")}
}
