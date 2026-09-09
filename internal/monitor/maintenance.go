package monitor

import (
	"context"
	"errors"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
)

func (m *Manager) maintenanceLoop(ctx context.Context) {
	timer := time.NewTimer(5 * time.Second)
	defer timer.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-timer.C:
			timer.Reset(m.maintenance(ctx, m.now()))
		}
	}
}

// Maintenance yields to foreground scans and never runs in the probe loop.
func (m *Manager) maintenance(ctx context.Context, now time.Time) time.Duration {
	if len(m.queries.slots) > 0 {
		return 10 * time.Second
	}
	if source, ok := m.source.(interface{ MonitorForegroundBusy() bool }); ok && source.MonitorForegroundBusy() {
		return 30 * time.Second
	}
	batch, cancel := context.WithTimeout(ctx, 250*time.Millisecond)
	processed, err := m.store.AggregateMonitorBatch(batch, now, 4)
	expired := batch.Err() != nil
	cancel()
	if err != nil {
		if ctx.Err() != nil || expired {
			return 10 * time.Second
		}
		m.storageFailure()
		return 30 * time.Second
	}
	delay := 30 * time.Second
	if processed > 0 {
		delay = 5 * time.Second
	}
	m.mu.Lock()
	clean := !now.Before(m.cleanAt)
	m.mu.Unlock()
	if clean {
		batch, cancel = context.WithTimeout(ctx, 250*time.Millisecond)
		err = m.store.CleanupMonitor(batch, now)
		expired = batch.Err() != nil
		cancel()
		if expired || ctx.Err() != nil {
			return 10 * time.Second
		}
		if errors.Is(err, history.ErrMonitorCleanupPending) {
			return 5 * time.Second
		}
		if err != nil {
			m.storageFailure()
			return 30 * time.Second
		}
		m.mu.Lock()
		m.cleanAt = now.Add(time.Hour)
		m.dataVersion++
		m.mu.Unlock()
	}
	return delay
}
