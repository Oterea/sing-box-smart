// Package gateway is the boundary for replacing the simulator with a sing-box adapter.
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
// A real adapter must classify ambiguous delay-endpoint errors with a separate health check.
