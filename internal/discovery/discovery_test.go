package discovery

import (
	"context"
	"errors"
	"sing-box-smart/internal/domain"
	"testing"
)

type client struct {
	airports []domain.Airport
	err      error
}

func (c client) Airports(context.Context) ([]domain.Airport, error) { return c.airports, c.err }
func (client) Probe(context.Context, string) (domain.Probe, error)  { panic("not used") }
func (client) Select(context.Context, string, string) error         { panic("not used") }
func (client) Current(context.Context, string) (string, error)      { panic("not used") }
func (client) Health(context.Context) error                         { panic("not used") }

func airport(id, selector string, nodes ...string) domain.Airport {
	a := domain.Airport{ID: id, Selector: selector}
	for _, n := range nodes {
		a.Nodes = append(a.Nodes, domain.Node{ID: n})
	}
	return a
}

func TestLoadValidatesCatalogInvariants(t *testing.T) {
	valid := []domain.Airport{airport("a", "a PIN", "1", "2"), airport("b", "b PIN", "3")}
	if got, err := Load(context.Background(), client{airports: valid}); err != nil || len(got) != 2 {
		t.Fatalf("valid catalog: %v %v", got, err)
	}
	cases := [][]domain.Airport{
		nil,
		{airport("a", "same", "1"), airport("a", "other", "2")},
		{airport("a", "same", "1"), airport("b", "same", "2")},
		{airport("a", "same")},
		{airport("a", "same", "1", "1")},
		{airport("", "same", "1")},
	}
	for i, airports := range cases {
		if _, err := Load(context.Background(), client{airports: airports}); err == nil {
			t.Fatalf("case %d accepted", i)
		}
	}
	if _, err := Load(context.Background(), client{err: errors.New("api down")}); err == nil {
		t.Fatal("gateway error swallowed")
	}
}
