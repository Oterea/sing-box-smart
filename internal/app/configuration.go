package app

import (
	"context"
	"fmt"
	"sing-box-smart/internal/discovery"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/gateway"
	"sing-box-smart/internal/real"
	"sing-box-smart/internal/settings"
)

func (a *App) prepareAPI(ctx context.Context, api string) (gateway.Client, []domain.Airport, string, error) {
	if a.cfg.Mode != "real" {
		return nil, nil, "", fmt.Errorf("模拟模式不支持修改真实 API")
	}
	api, err := settings.NormalizeAPI(api)
	if err != nil {
		return nil, nil, "", err
	}
	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		return nil, nil, "", err
	}
	cfg := a.cfg
	cfg.API, cfg.Root, cfg.Pattern = api, snapshot.GroupRoot, snapshot.GroupPattern
	client := real.New(cfg)
	airports, err := discovery.Load(ctx, client)
	if err != nil {
		return nil, nil, "", err
	}
	return client, airports, api, nil
}

// ConfigureConnection validates and discovers the complete connection in one
// step, so an API/root/pattern change cannot leave a half-applied configuration.
func (a *App) ConfigureConnection(ctx context.Context, api, root, pattern string) error {
	if a.cfg.Mode != "real" {
		return fmt.Errorf("模拟模式不支持修改真实 API")
	}
	api, err := settings.NormalizeAPI(api)
	if err != nil {
		return err
	}
	if root == "" {
		root = "proxy"
	}
	pattern, err = settings.NormalizePattern(pattern)
	if err != nil {
		return err
	}
	cfg := a.cfg
	cfg.API, cfg.Root, cfg.Pattern = api, root, pattern
	client := real.New(cfg)
	airports, err := discovery.Load(ctx, client)
	if err != nil {
		return err
	}
	_, err = a.request(ctx, request{action: "api", target: api, root: root, pattern: pattern, client: client, airports: airports})
	return err
}

func (a *App) TestAPI(ctx context.Context, api string) error {
	_, _, _, err := a.prepareAPI(ctx, api)
	return err
}
func (a *App) ConfigureAPI(ctx context.Context, api string) error {
	client, airports, api, err := a.prepareAPI(ctx, api)
	if err != nil {
		return err
	}
	_, err = a.request(ctx, request{action: "api", target: api, client: client, airports: airports})
	return err
}
func (a *App) ConfigureFilters(ctx context.Context, root, pattern string) error {
	if root == "" {
		root = "proxy"
	}
	if _, err := settings.NormalizePattern(pattern); err != nil {
		return err
	}
	snapshot, err := a.Snapshot(ctx)
	if err != nil {
		return err
	}
	cfg := a.cfg
	cfg.Root = root
	cfg.Pattern = pattern
	cfg.API = snapshot.APIAddress
	client := real.New(cfg)
	airports, err := discovery.Load(ctx, client)
	if err != nil {
		return err
	}
	_, err = a.request(ctx, request{action: "api", target: snapshot.APIAddress, root: root, pattern: pattern, client: client, airports: airports})
	return err
}
