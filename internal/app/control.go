package app

import (
	"context"
	"fmt"
	"sing-box-smart/internal/domain"
	"sing-box-smart/internal/settings"
	"time"
)

func (a *App) configureAPI(r request) error {
	if a.switchBusy || a.pending != "" || a.healthBusy {
		return fmt.Errorf("切换正在进行，请稍后重试")
	}
	root, pattern := a.groupRoot, a.groupPattern
	if r.root != "" {
		root, pattern = r.root, r.pattern
	}
	if err := settings.Save(a.cfg.SettingsPath, settings.Connection{API: r.target, Root: root, Pattern: pattern}); err != nil {
		return fmt.Errorf("保存配置失败: %w", err)
	}
	a.apiAddress = r.target
	a.groupRoot, a.groupPattern = root, pattern
	selected := a.airport.ID
	a.client = r.client
	a.runner.Client = r.client
	a.airports = r.airports
	a.healthy = true
	a.rootSelection = ""
	a.controlActive = false
	a.lastSelectionSync = time.Time{}
	for _, airport := range a.airports {
		if airport.ID == selected {
			a.reset(airport)
			return nil
		}
	}
	a.reset(a.airports[0])
	return nil
}

func (a *App) control(ctx context.Context, action, target string) error {
	if action == "pause" {
		a.pauseDetection()
		return nil
	}
	if action == "resume" {
		a.resumeDetection()
		return nil
	}
	if action == "auto" || action == "manual" {
		return a.controlSelectionMode(ctx, action)
	}
	if a.paused {
		return fmt.Errorf("检测已暂停，请先继续检测")
	}
	if action == "recheck" {
		a.revision++
		for _, n := range a.store.Nodes {
			n.Plan.ClearRecovery()
			n.Plan.NormalDue = time.Now()
		}
		a.events.Record("manual", "已安排当前机场全部节点重新检查")
		return nil
	}
	if !a.healthy {
		return fmt.Errorf("管理接口暂不可用")
	}
	if a.pending != "" || a.switchBusy {
		return fmt.Errorf("已有切换正在处理中")
	}
	switch action {
	case "node":
		if a.phase == "startup" {
			return fmt.Errorf("启动检查尚未结束")
		}
		if a.store.ByID[target] == nil {
			return fmt.Errorf("节点不属于当前机场")
		}
		if !a.controlActive || !a.selectionInitialized {
			return fmt.Errorf("请先在根策略组选择 smart 策略组")
		}
		a.setSelectionMode(domain.SelectionManual)
		if target == a.current {
			return nil
		}
		a.pending = target
		a.pendingKind = "node"
		a.revision++
		a.pendingManual = true
		a.selectionEpoch++
		a.events.Record("manual", "手动选择：先检查目标节点")
		a.startProbe(ctx, a.store.ByID[target], "confirm")
	case "airport":
		for _, airport := range a.airports {
			if airport.ID == target {
				if target == a.airport.ID && a.controlActive {
					return nil
				}
				a.pending = target
				a.pendingKind = "airport"
				a.selectionEpoch++
				a.revision++
				a.apply(ctx, "airport", target, a.groupRoot, airport.Selector)
				return nil
			}
		}
		return fmt.Errorf("未知机场")
	default:
		return fmt.Errorf("未知操作")
	}
	return nil
}
