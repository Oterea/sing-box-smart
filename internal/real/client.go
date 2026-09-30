// Package real implements the Clash-compatible management API exposed by sing-box.
package real

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net"
	"net/http"
	"net/url"
	"sing-box-smart/internal/config"
	"sing-box-smart/internal/domain"
	"strconv"
	"strings"
	"sync"
	"time"
)

type Client struct {
	mu                  sync.RWMutex
	base, root, testURL string
	pins                []config.Pin
	hc                  *http.Client
}

func (c *Client) SetAPI(api string) error {
	u, err := url.Parse(strings.TrimRight(api, "/"))
	if err != nil || u.Scheme == "" || u.Host == "" || (u.Scheme != "http" && u.Scheme != "https") {
		return fmt.Errorf("API 地址无效")
	}
	c.mu.Lock()
	c.base = strings.TrimRight(api, "/")
	c.mu.Unlock()
	return nil
}
func (c *Client) endpoint(path string) string {
	c.mu.RLock()
	defer c.mu.RUnlock()
	return c.base + path
}

type proxyInfo struct {
	Type string   `json:"type"`
	Name string   `json:"name"`
	Now  string   `json:"now"`
	All  []string `json:"all"`
}
type apiError struct {
	Message string `json:"message"`
}

func New(c config.Config) *Client {
	return &Client{base: strings.TrimRight(c.API, "/"), root: c.Root, testURL: c.TestURL, pins: c.Pins, hc: &http.Client{Transport: &http.Transport{MaxIdleConns: 64, MaxIdleConnsPerHost: 64, IdleConnTimeout: 90 * time.Second, DialContext: (&net.Dialer{Timeout: 3 * time.Second}).DialContext}}}
}
func (c *Client) get(ctx context.Context, path string, dst any) error {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint(path), nil)
	if err != nil {
		return err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("sing-box API unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode < 200 || resp.StatusCode >= 300 {
		return c.httpError(resp)
	}
	if dst != nil {
		if err := json.NewDecoder(resp.Body).Decode(dst); err != nil {
			return fmt.Errorf("decode %s: %w", path, err)
		}
	}
	return nil
}
func (c *Client) httpError(resp *http.Response) error {
	var e apiError
	_ = json.NewDecoder(resp.Body).Decode(&e)
	if e.Message != "" {
		return fmt.Errorf("HTTP %d: %s", resp.StatusCode, e.Message)
	}
	return fmt.Errorf("HTTP %d", resp.StatusCode)
}
func (c *Client) members(ctx context.Context, group string) ([]string, string, error) {
	var info proxyInfo
	err := c.get(ctx, "/proxies/"+url.PathEscape(group), &info)
	return info.All, info.Now, err
}
func (c *Client) Airports(ctx context.Context) ([]domain.Airport, error) {
	out := make([]domain.Airport, 0, len(c.pins))
	for _, pin := range c.pins {
		members, _, err := c.members(ctx, pin.Selector)
		if err != nil {
			// A configured PIN may be absent while momo/sing-box is reloading.
			// Keep the other selectors usable and let the UI show the available set.
			continue
		}
		a := domain.Airport{ID: pin.ID, Name: pin.Name, Selector: pin.Selector}
		for _, name := range members {
			if name != "" {
				a.Nodes = append(a.Nodes, domain.Node{ID: name, Name: name})
			}
		}
		if len(a.Nodes) == 0 {
			return nil, fmt.Errorf("selector %q has no nodes", pin.Selector)
		}
		out = append(out, a)
	}
	if len(out) == 0 {
		return nil, fmt.Errorf("no configured selectors available")
	}
	return out, nil
}
func (c *Client) Probe(ctx context.Context, node string) (domain.Probe, error) {
	timeout := 4 * time.Second
	if d, ok := ctx.Deadline(); ok {
		timeout = time.Until(d)
		if timeout < 100*time.Millisecond {
			timeout = 100 * time.Millisecond
		}
	}
	// Keep a small outer margin so sing-box can return its normal delay error.
	// That response is a node failure; an outer context timeout is also treated
	// as a node failure because the request was already sent to the API.
	innerTimeout := timeout - 500*time.Millisecond
	if innerTimeout < 500*time.Millisecond {
		innerTimeout = 500 * time.Millisecond
	}
	q := url.Values{"url": {c.testURL}, "timeout": {strconv.FormatInt(innerTimeout.Milliseconds(), 10)}}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.endpoint("/proxies/")+url.PathEscape(node)+"/delay?"+q.Encode(), nil)
	if err != nil {
		return domain.Probe{}, err
	}
	resp, err := c.hc.Do(req)
	if err != nil {
		if errors.Is(err, context.DeadlineExceeded) || errors.Is(ctx.Err(), context.DeadlineExceeded) {
			return domain.Probe{At: time.Now(), Error: "探测超时"}, nil
		}
		return domain.Probe{}, fmt.Errorf("sing-box API unreachable: %w", err)
	}
	defer resp.Body.Close()
	var body struct {
		Delay   int    `json:"delay"`
		Message string `json:"message"`
	}
	_ = json.NewDecoder(resp.Body).Decode(&body)
	if resp.StatusCode != http.StatusOK || body.Delay <= 0 {
		msg := body.Message
		if msg == "" {
			msg = fmt.Sprintf("HTTP %d", resp.StatusCode)
		}
		return domain.Probe{At: time.Now(), Error: msg}, nil
	}
	return domain.Probe{At: time.Now(), Success: true, DelayMS: float64(body.Delay)}, nil
}
func (c *Client) Select(ctx context.Context, group, target string) error {
	body, _ := json.Marshal(map[string]string{"name": target})
	req, err := http.NewRequestWithContext(ctx, http.MethodPut, c.endpoint("/proxies/"+url.PathEscape(group)), bytes.NewReader(body))
	if err != nil {
		return err
	}
	req.Header.Set("Content-Type", "application/json")
	resp, err := c.hc.Do(req)
	if err != nil {
		return fmt.Errorf("sing-box API unreachable: %w", err)
	}
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusNoContent {
		return c.httpError(resp)
	}
	return nil
}
func (c *Client) Current(ctx context.Context, group string) (string, error) {
	_, now, err := c.members(ctx, group)
	return now, err
}
func (c *Client) Health(ctx context.Context) error { _, _, err := c.members(ctx, c.root); return err }
