package probe

import (
	"context"
	"errors"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/gateway"
	"testing"
	"time"
)

type probeClient struct {
	fn func(context.Context, string) (domain.Probe, error)
}

var _ gateway.Client = probeClient{}

func (probeClient) Airports(context.Context) ([]domain.Airport, error) { return nil, nil }
func (probeClient) Select(context.Context, string, string) error       { return nil }
func (probeClient) Current(context.Context, string) (string, error)    { return "", nil }
func (probeClient) Health(context.Context) error                       { return nil }

func (p probeClient) Probe(ctx context.Context, id string) (domain.Probe, error) {
	return p.fn(ctx, id)
}

func TestRunnerBoundsSlowProbe(t *testing.T) {
	r := Runner{Timeout: 10 * time.Millisecond, Client: probeClient{fn: func(ctx context.Context, _ string) (domain.Probe, error) {
		<-ctx.Done()
		return domain.Probe{}, ctx.Err()
	}}}
	started := time.Now()
	_, err := r.Run(context.Background(), "a")
	if !errors.Is(err, context.DeadlineExceeded) || time.Since(started) > 200*time.Millisecond {
		t.Fatalf("err=%v elapsed=%v", err, time.Since(started))
	}
}

func TestRunnerPropagatesClientResultAndParentCancellation(t *testing.T) {
	want := domain.Probe{Success: true, DelayMS: 42}
	r := Runner{Timeout: time.Second, Client: probeClient{fn: func(context.Context, string) (domain.Probe, error) { return want, nil }}}
	got, err := r.Run(context.Background(), "a")
	if err != nil || got != want {
		t.Fatalf("got=%+v err=%v", got, err)
	}
	ctx, cancel := context.WithCancel(context.Background())
	cancel()
	r.Client = probeClient{fn: func(ctx context.Context, _ string) (domain.Probe, error) {
		<-ctx.Done()
		return domain.Probe{}, ctx.Err()
	}}
	_, err = r.Run(ctx, "a")
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("parent cancellation err=%v", err)
	}
}
