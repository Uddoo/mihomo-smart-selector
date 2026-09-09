package monitor

import (
	"context"
	"errors"
	"sync"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

var ErrHistoryBusy = errors.New("历史查询繁忙，请稍后刷新；日常监控和自动切换继续运行")

type viewEntry struct {
	value   model.MonitorOverview
	until   time.Time
	version uint64
}
type viewFlight struct{ done chan struct{} }
type queryBudget struct {
	mu      sync.Mutex
	cache   map[string]viewEntry
	flights map[string]*viewFlight
	slots   chan struct{}
}

func newQueryBudget() *queryBudget {
	return &queryBudget{cache: map[string]viewEntry{}, flights: map[string]*viewFlight{}, slots: make(chan struct{}, 1)}
}

func (m *Manager) acquireHistory(ctx context.Context) (func(), error) {
	select {
	case m.queries.slots <- struct{}{}:
		return func() { <-m.queries.slots }, nil
	default:
	}
	timer := time.NewTimer(250 * time.Millisecond)
	defer timer.Stop()
	select {
	case m.queries.slots <- struct{}{}:
		return func() { <-m.queries.slots }, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	case <-timer.C:
		return nil, ErrHistoryBusy
	}
}

func cloneOverview(v model.MonitorOverview) model.MonitorOverview {
	v.Plan = clonePlan(v.Plan)
	v.Events = append([]model.MonitorEvent{}, v.Events...)
	v.Rows = append([]model.MonitorRow{}, v.Rows...)
	for i := range v.Rows {
		v.Rows[i].Series = append([]model.MonitorSample{}, v.Rows[i].Series...)
		if v.Rows[i].Metrics.Score != nil {
			score := *v.Rows[i].Metrics.Score
			v.Rows[i].Metrics.Score = &score
		}
	}
	return v
}

func (m *Manager) cachedOverview(ctx context.Context, window string) (model.MonitorOverview, error) {
	if _, err := WindowDuration(window); err != nil {
		return model.MonitorOverview{}, err
	}
	if window == "" {
		window = "24h"
	}
	for {
		m.mu.Lock()
		version := m.dataVersion
		now := m.now()
		m.mu.Unlock()
		q := m.queries
		q.mu.Lock()
		if entry, ok := q.cache[window]; ok && entry.version == version && now.Before(entry.until) {
			v := cloneOverview(entry.value)
			q.mu.Unlock()
			return v, nil
		}
		if flight, ok := q.flights[window]; ok {
			done := flight.done
			q.mu.Unlock()
			select {
			case <-ctx.Done():
				return model.MonitorOverview{}, ctx.Err()
			case <-done:
				continue
			}
		}
		flight := &viewFlight{done: make(chan struct{})}
		q.flights[window] = flight
		q.mu.Unlock()
		release, err := m.acquireHistory(ctx)
		var value model.MonitorOverview
		if err == nil {
			value, err = m.overview(ctx, window, true)
			release()
		}
		q.mu.Lock()
		if err == nil {
			q.cache[window] = viewEntry{value: cloneOverview(value), version: value.DataVersion, until: now.Add(10 * time.Second)}
		}
		delete(q.flights, window)
		close(flight.done)
		q.mu.Unlock()
		return value, err
	}
}
