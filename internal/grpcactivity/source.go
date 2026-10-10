// Package grpcactivity adapts sing-box's native API connection stream to the
// small activity interface used by the sleep controller.
package grpcactivity

import (
	"context"
	"strings"
	"time"

	"google.golang.org/grpc"
	"google.golang.org/grpc/credentials"
	"google.golang.org/grpc/credentials/insecure"

	"sing-box-smart/internal/activity"
	daemon "sing-box-smart/internal/singboxapi/daemon"
)

type Config struct {
	Address      string
	Secret       string
	Interval     time.Duration
	ProbeDomains []string
}

type Source struct{ cfg Config }

func New(cfg Config) *Source {
	if cfg.Interval <= 0 {
		cfg.Interval = time.Second
	}
	return &Source{cfg: cfg}
}

type tokenCredentials string

func (t tokenCredentials) GetRequestMetadata(context.Context, ...string) (map[string]string, error) {
	return map[string]string{"authorization": "Bearer " + string(t)}, nil
}
func (tokenCredentials) RequireTransportSecurity() bool { return false }

func (s *Source) dialOptions() []grpc.DialOption {
	opts := []grpc.DialOption{grpc.WithTransportCredentials(insecure.NewCredentials())}
	if strings.TrimSpace(s.cfg.Secret) != "" {
		opts = append(opts, grpc.WithPerRPCCredentials(tokenCredentials(s.cfg.Secret)))
	}
	return opts
}

// Run reconnects forever.  A gRPC outage is intentionally not converted to
// user inactivity; the caller therefore never enters sleep just because the
// API disappeared.
func (s *Source) Run(ctx context.Context, sink chan<- activity.Event) error {
	if strings.TrimSpace(s.cfg.Address) == "" {
		return nil
	}
	backoff := time.Second
	for {
		dialCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		conn, err := grpc.DialContext(dialCtx, s.cfg.Address, append(s.dialOptions(), grpc.WithBlock())...)
		cancel()
		if err == nil {
			err = s.consume(ctx, conn, sink)
			_ = conn.Close()
		}
		if ctx.Err() != nil {
			return ctx.Err()
		}
		timer := time.NewTimer(backoff)
		select {
		case <-ctx.Done():
			if !timer.Stop() {
				<-timer.C
			}
			return ctx.Err()
		case <-timer.C:
		}
		if backoff < 30*time.Second {
			backoff *= 2
		}
		_ = err
	}
}

func (s *Source) consume(ctx context.Context, conn *grpc.ClientConn, sink chan<- activity.Event) error {
	client := daemon.NewStartedServiceClient(conn)
	stream, err := client.SubscribeConnections(ctx, &daemon.SubscribeConnectionsRequest{Interval: s.cfg.Interval.Milliseconds()})
	if err != nil {
		return err
	}
	for {
		batch, err := stream.Recv()
		if err != nil {
			return err
		}
		for _, event := range batch.GetEvents() {
			if event == nil {
				continue
			}
			mapped := activity.Event{At: time.Now(), ID: event.GetId(), UplinkDelta: event.GetUplinkDelta(), DownlinkDelta: event.GetDownlinkDelta()}
			switch event.GetType() {
			case daemon.ConnectionEventType_CONNECTION_EVENT_NEW:
				mapped.Kind = activity.ConnectionNew
			case daemon.ConnectionEventType_CONNECTION_EVENT_UPDATE:
				mapped.Kind = activity.ConnectionUpdate
			case daemon.ConnectionEventType_CONNECTION_EVENT_CLOSED:
				mapped.Kind = activity.ConnectionClosed
			}
			if c := event.GetConnection(); c != nil {
				mapped.Domain = c.GetDomain()
				mapped.Destination = c.GetDestination()
				mapped.Inbound = c.GetInbound()
				mapped.InboundType = c.GetInboundType()
				mapped.Probe = s.isProbe(c.GetDomain(), c.GetDestination())
			}
			select {
			case sink <- mapped:
			case <-ctx.Done():
				return ctx.Err()
			}
		}
	}
}

func (s *Source) isProbe(domain, destination string) bool {
	for _, host := range s.cfg.ProbeDomains {
		host = strings.ToLower(strings.TrimSpace(host))
		if host != "" && (strings.EqualFold(domain, host) || strings.HasPrefix(strings.ToLower(destination), host+":")) {
			return true
		}
	}
	return false
}

var _ credentials.PerRPCCredentials = tokenCredentials("")
var _ activity.Source = (*Source)(nil)
