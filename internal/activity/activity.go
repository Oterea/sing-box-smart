// Package activity contains the transport-neutral events used by sleep monitoring.
package activity

import (
	"context"
	"time"
)

type Kind uint8

const (
	ConnectionNew Kind = iota
	ConnectionUpdate
	ConnectionClosed
)

// Event is deliberately smaller than sing-box's connection model.  Sleep
// monitoring only needs to know whether a real user connection was active.
type Event struct {
	Kind          Kind
	ID            string
	Domain        string
	Destination   string
	Inbound       string
	InboundType   string
	UplinkDelta   int64
	DownlinkDelta int64
	At            time.Time
	Probe         bool
}

type Source interface {
	Run(ctx context.Context, sink chan<- Event) error
}
