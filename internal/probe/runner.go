// Package probe executes one bounded check. Scheduling and score updates are owned elsewhere.
package probe

import (
	"context"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/gateway"
	"time"
)

type Runner struct {
	Client  gateway.Client
	Timeout time.Duration
}

func (r Runner) Run(ctx context.Context, id string) (domain.Probe, error) {
	ctx, cancel := context.WithTimeout(ctx, r.Timeout)
	defer cancel()
	return r.Client.Probe(ctx, id)
}
