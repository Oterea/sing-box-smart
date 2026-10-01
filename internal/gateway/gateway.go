// Package gateway defines the interface shared by real and demo adapters.
package gateway

import (
	"context"
	"sing-box-smart/internal/domain"
)

type Client interface {
	Airports(context.Context) ([]domain.Airport, error)
	Probe(context.Context, string) (domain.Probe, error)
	Select(context.Context, string, string) error
	Current(context.Context, string) (string, error)
	Health(context.Context) error
}

// APIConfigurable is implemented by gateways whose management endpoint can
// be changed while the monitor is running.
type APIConfigurable interface{ SetAPI(string) error }

// Probe returns node failures as Probe{Success:false}, and management/transport failures as error.
// The real adapter treats HTTP delay errors and deadline timeouts as node
// failures; other transport errors are API errors. No per-response health check
// is performed.
