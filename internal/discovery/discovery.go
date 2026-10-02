package discovery

import (
	"context"
	"fmt"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/gateway"
)

func Load(ctx context.Context, client gateway.Client) ([]domain.Airport, error) {
	airports, err := client.Airports(ctx)
	if err != nil {
		return nil, err
	}
	if len(airports) == 0 {
		return nil, fmt.Errorf("no airport selectors found")
	}
	ids := map[string]bool{}
	selectors := map[string]bool{}
	for _, a := range airports {
		if a.ID == "" || a.Selector == "" || len(a.Nodes) == 0 || ids[a.ID] || selectors[a.Selector] {
			return nil, fmt.Errorf("invalid or duplicate airport")
		}
		ids[a.ID] = true
		selectors[a.Selector] = true
		nodes := map[string]bool{}
		for _, n := range a.Nodes {
			if n.ID == "" || nodes[n.ID] {
				return nil, fmt.Errorf("invalid or duplicate node in %s", a.ID)
			}
			nodes[n.ID] = true
		}
	}
	return airports, nil
}
