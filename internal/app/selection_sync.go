package app

import (
	"context"
	"sing-box-smart/internal/domain"
	"time"
)

// One app-owned reader serves every browser, even while detection is paused.
const selectionSyncInterval = 2 * time.Second

type selectionObservation struct {
	root, node string
	generation int
	epoch      uint64
	err        error
}

func (a *App) startSelectionSync(ctx context.Context) {
	if a.selectionSyncBusy || a.client == nil || a.switchBusy || a.pending != "" {
		return
	}
	a.selectionSyncBusy = true
	client, root := a.client, a.groupRoot
	airports := append([]domain.Airport(nil), a.airports...)
	generation, epoch, timeout := a.generation, a.selectionEpoch, a.cfg.Timeout
	go func() {
		c, cancel := context.WithTimeout(ctx, timeout)
		defer cancel()
		rootNow, err := client.Current(c, root)
		nodeNow := ""
		if err == nil {
			for _, airport := range airports {
				if airport.Selector == rootNow {
					nodeNow, err = client.Current(c, airport.Selector)
					break
				}
			}
		}
		select {
		case a.selection <- selectionObservation{root: rootNow, node: nodeNow, generation: generation, epoch: epoch, err: err}:
		case <-ctx.Done():
		}
	}()
}

func (a *App) applySelectionObservation(o selectionObservation) {
	if o.generation != a.generation || o.epoch != a.selectionEpoch || a.pending != "" || a.switchBusy {
		return
	}
	if o.err != nil || o.root == "" {
		// Unknown current selection cannot safely authorize automatic writes.
		if a.controlActive {
			a.controlActive = false
			a.revision++
		}
		return
	}
	initial := a.rootSelection == ""
	rootChanged := a.rootSelection != o.root
	if rootChanged {
		a.rootSelection = o.root
		a.revision++
	}
	airport, ok := a.airportBySelector(o.root)
	if !ok {
		if a.controlActive {
			a.controlActive = false
			a.revision++
		}
		if rootChanged && !initial {
			a.setSelectionMode(domain.SelectionManual)
			a.events.Record("manual", "根策略组已选择 "+o.root+"，smart 停止自动切换，继续检测")
		}
		return
	}
	if !a.controlActive {
		a.controlActive = true
		a.revision++
	}
	if airport.ID != a.airport.ID {
		a.reset(airport)
		if !initial {
			a.setSelectionMode(domain.SelectionManual)
			a.events.Record("manual", "检测到 sing-box 外部切换策略组："+airport.Selector)
		}
	} else if rootChanged && !initial {
		a.setSelectionMode(domain.SelectionManual)
	}
	if o.node == "" || a.store.ByID[o.node] == nil {
		a.controlActive = false
		a.revision++
		return
	}
	if !a.selectionInitialized {
		a.setCurrentNode(o.node)
		a.selectionInitialized = true
	} else if o.node != a.current {
		a.setSelectionMode(domain.SelectionManual)
		a.setCurrentNode(o.node)
		a.events.Record("manual", "检测到 sing-box 外部切换节点："+o.node)
	}
	a.evaluate(a.rootCtx)
}

// All confirmed node changes use the same classification and scheduling path.
func (a *App) setCurrentNode(id string) {
	if a.current == id {
		return
	}
	a.current = id
	a.store.Reclassify(id)
	if a.phase != "startup" && a.phase != "refresh" {
		a.store.Reschedule(time.Now(), id, a.cfg.Current, a.cfg.Candidate, a.cfg.Ordinary)
	}
	a.revision++
}

func (a *App) airportBySelector(selector string) (domain.Airport, bool) {
	for _, airport := range a.airports {
		if airport.Selector == selector {
			return airport, true
		}
	}
	return domain.Airport{}, false
}
