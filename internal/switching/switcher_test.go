package switching

import (
	"context"
	"errors"
	"sing-box-smart/internal/domain"
	"testing"
)

type fakeClient struct {
	selected   string
	selectErr  error
	currentErr error
}

func (f *fakeClient) Airports(context.Context) ([]domain.Airport, error)  { return nil, nil }
func (f *fakeClient) Probe(context.Context, string) (domain.Probe, error) { return domain.Probe{}, nil }
func (f *fakeClient) Select(_ context.Context, _, target string) error {
	if f.selectErr == nil {
		f.selected = target
	}
	return f.selectErr
}
func (f *fakeClient) Current(context.Context, string) (string, error) {
	return f.selected, f.currentErr
}
func (f *fakeClient) Health(context.Context) error { return nil }

func TestApplyVerifiesReadback(t *testing.T) {
	f := &fakeClient{selected: "old"}
	r := Apply(context.Background(), f, "group", "new")
	if !r.Verified || r.Actual != "new" {
		t.Fatalf("result=%+v", r)
	}
}

func TestApplyReportsReadbackMismatch(t *testing.T) {
	f := &fakeClient{selected: "old"}
	f.selectErr = errors.New("rejected")
	r := Apply(context.Background(), f, "group", "new")
	if r.Verified || r.Actual != "old" || r.Err == nil {
		t.Fatalf("result=%+v", r)
	}
}
