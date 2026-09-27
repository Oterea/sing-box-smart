// Package demo supplies deterministic synthetic network behavior through the same gateway interface.
package demo

import (
	"context"
	"fmt"
	"sing-box-smart/internal/domain"
	"sync"
	"time"
)

type profile struct {
	delay float64
	index int
}
type Client struct {
	mu       sync.Mutex
	started  time.Time
	airports []domain.Airport
	nodes    map[string]profile
	counts   map[string]int
	selected map[string]string
}

func New() *Client {
	c := &Client{started: time.Now(), nodes: map[string]profile{}, counts: map[string]int{}, selected: map[string]string{}}
	names := []string{"香港 · 中环 01", "日本 · 东京 02", "新加坡 · 滨海 03", "台湾 · 台北 04", "香港 · 九龙 05", "日本 · 大阪 06", "美国 · 洛杉矶 07", "德国 · 法兰克福 08", "新加坡 · 裕廊 09", "韩国 · 首尔 10", "英国 · 伦敦 11", "美国 · 西雅图 12"}
	delays := []float64{58, 92, 126, 79, 72, 113, 186, 238, 145, 103, 269, 211}
	for ai, name := range []string{"Pei", "ToLink", "Cloud"} {
		id := fmt.Sprintf("airport-%d", ai+1)
		a := domain.Airport{ID: id, Name: name, Selector: name + " PIN"}
		for i, n := range names {
			node := domain.Node{ID: fmt.Sprintf("%s-node-%02d", id, i+1), Name: n}
			a.Nodes = append(a.Nodes, node)
			c.nodes[node.ID] = profile{delays[i] + float64(ai*15), i}
		}
		c.airports = append(c.airports, a)
		c.selected[a.Selector] = a.Nodes[0].ID
	}
	c.selected["proxy"] = c.airports[0].Selector
	return c
}
func (c *Client) Airports(ctx context.Context) ([]domain.Airport, error) {
	return c.airports, ctx.Err()
}
func (c *Client) Health(ctx context.Context) error { return ctx.Err() }
func (c *Client) Probe(ctx context.Context, id string) (domain.Probe, error) {
	c.mu.Lock()
	p, ok := c.nodes[id]
	c.counts[id]++
	count := c.counts[id]
	elapsed := time.Since(c.started)
	c.mu.Unlock()
	if !ok {
		return domain.Probe{}, fmt.Errorf("unknown node")
	}
	delay := p.delay + float64((count*17+p.index*7)%31-15)
	failed := p.index == 8 && count%3 == 0 || p.index == 10 && count%4 == 0 || p.index == 0 && int(elapsed.Seconds())%90 >= 35 && int(elapsed.Seconds())%90 < 55
	wait := time.Duration(delay) * time.Millisecond
	if failed {
		wait = 650 * time.Millisecond
	}
	timer := time.NewTimer(wait)
	defer timer.Stop()
	select {
	case <-ctx.Done():
		return domain.Probe{}, ctx.Err()
	case <-timer.C:
	}
	r := domain.Probe{Success: !failed, DelayMS: delay, At: time.Now()}
	if failed {
		r.Error = "模拟连接失败"
	}
	return r, nil
}
func (c *Client) Select(ctx context.Context, group, target string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	valid := false
	for _, a := range c.airports {
		if group == "proxy" && target == a.Selector {
			valid = true
		}
		if group == a.Selector {
			for _, n := range a.Nodes {
				if n.ID == target {
					valid = true
				}
			}
		}
	}
	if !valid {
		return fmt.Errorf("target does not belong to selector")
	}
	c.selected[group] = target
	return nil
}
func (c *Client) Current(ctx context.Context, group string) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	v, ok := c.selected[group]
	if !ok {
		return "", fmt.Errorf("unknown selector")
	}
	return v, ctx.Err()
}
