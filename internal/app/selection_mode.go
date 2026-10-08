package app

import (
	"context"
	"fmt"
	"sing-box-smart/internal/domain"
	"time"
)

// Mode changes affect selection only; probe scheduling and scoring stay active.
func (a *App) setSelectionMode(mode domain.SelectionMode) {
	if a.selectionMode == mode {
		return
	}
	a.selectionMode = mode
	a.selectionEpoch++
	a.revision++
}

func (a *App) controlSelectionMode(ctx context.Context, action string) error {
	if a.switchBusy {
		return fmt.Errorf("切换写入正在进行，请稍后重试")
	}
	if action == "manual" {
		// A confirmation probe may finish and score normally, but must not
		// authorize an automatic write after the user asks to hold the node.
		a.pending, a.pendingKind = "", ""
		a.pendingManual = false
		a.selectionEpoch++
		a.setSelectionMode(domain.SelectionManual)
		a.revision++
		a.lastSelectionSync = time.Time{}
		a.events.Record("manual", "已开启手动选择，保持 sing-box 当前节点，继续检测")
		return nil
	}
	if a.pending != "" {
		return fmt.Errorf("已有切换正在确认，请稍后重试")
	}
	a.setSelectionMode(domain.SelectionAuto)
	a.selectionInitialized = false
	a.lastSelectionSync = time.Time{}
	a.events.Record("manual", "已开启自动选择，将同步当前节点后按分数判断")
	// Wait for a fresh selector read before evaluating; stale UI state must
	// never overwrite a selection just made in another dashboard.
	a.selectionEpoch++
	a.startSelectionSync(ctx)
	return nil
}
