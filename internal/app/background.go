package app

import (
	"context"
	"sing-box-smart/internal/background"
	"sing-box-smart/internal/domain"
	"time"
)

// resetBackgroundSchedule invalidates pending background probes whenever the
// shared probe context is replaced. It preserves observations and history but
// starts a fresh staggered schedule on the next tick.
func (a *App) resetBackgroundSchedule() {
	a.backgroundScheduleInterval = 0
	for _, group := range a.backgroundGroups {
		group.Due = map[string]time.Time{}
		group.InFlight = map[string]bool{}
	}
}

func (a *App) reconcileBackgroundGroups(airports []domain.Airport) {
	seen := make(map[string]bool, len(airports))
	for _, airport := range airports {
		seen[airport.ID] = true
		group := a.backgroundGroups[airport.ID]
		if group == nil {
			group = background.New(airport, a.cfg.HistoryLimit)
			a.backgroundGroups[airport.ID] = group
		} else {
			group.UpdateAirport(airport, a.cfg.HistoryLimit)
		}
		currentCtx, cancel := context.WithTimeout(a.rootCtx, a.cfg.Timeout)
		current, err := a.client.Current(currentCtx, airport.Selector)
		cancel()
		if err == nil && group.Store.ByID[current] != nil {
			group.CurrentID = current
		} else if err == nil {
			group.CurrentID = ""
		}
	}
	for id := range a.backgroundGroups {
		if !seen[id] {
			delete(a.backgroundGroups, id)
		}
	}
}
