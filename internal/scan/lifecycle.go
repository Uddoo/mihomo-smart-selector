package scan

import (
	"context"
	"fmt"
	"log"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Called under settingsMu, so admission and insertion cannot race another Start.
func (m *Manager) admitScan(group string) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	running := 0
	for _, s := range m.active {
		if s.Status == model.ScanRunning {
			running++
			if s.Request.TargetGroup == group {
				return fmt.Errorf("该策略组已有扫描，请恢复或结束现有任务")
			}
		}
	}
	if running >= m.currentConfig().Scanner.MaxActiveScans {
		return fmt.Errorf("已达到同时扫描上限，请等待现有任务结束")
	}
	return nil
}

func (m *Manager) Recent(ctx context.Context) ([]model.Scan, error) {
	items, err := m.store.RecentScans(ctx, 20)
	if err != nil {
		return nil, err
	}
	m.mu.Lock()
	defer m.mu.Unlock()
	seen := map[string]bool{}
	for i := range items {
		seen[items[i].ID] = true
		if active, ok := m.active[items[i].ID]; ok {
			items[i] = cloneScan(*active)
		}
		items[i].Results = nil
	}
	for id, active := range m.active {
		if !seen[id] {
			item := cloneScan(*active)
			item.Results = nil
			items = append(items, item)
		}
	}
	sort.SliceStable(items, func(i, j int) bool {
		a, b := items[i], items[j]
		if (a.Status == model.ScanRunning) != (b.Status == model.ScanRunning) {
			return a.Status == model.ScanRunning
		}
		return a.StartedAt.After(b.StartedAt)
	})
	return items, nil
}

func (m *Manager) Storage(ctx context.Context) (history.StorageStats, error) {
	return m.store.Stats(ctx)
}
func (m *Manager) Cleanup(ctx context.Context, revision int) (history.CleanupResult, error) {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	if revision != m.settingsRevision {
		return history.CleanupResult{}, fmt.Errorf("保留策略已更新，请重新确认清理")
	}
	if m.hasRunningScan() {
		return history.CleanupResult{}, fmt.Errorf("扫描运行时不能清理历史")
	}
	return m.store.MaintainHistory(ctx, m.currentConfig().Storage.Retention)
}

func (m *Manager) StartMaintenance() {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	if m.maintenanceCancel != nil || m.stopping {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.maintenanceCancel = cancel
	m.workers.Add(1)
	go func() {
		defer m.workers.Done()
		ticker := time.NewTicker(time.Hour)
		defer ticker.Stop()
		for {
			if m.settingsMu.TryLock() {
				ready := !m.hasRunningScan() && !m.stopping
				retention := m.currentConfig().Storage.Retention
				m.settingsMu.Unlock()
				if ready {
					if _, err := m.store.MaintainHistory(ctx, retention); err != nil && ctx.Err() == nil {
						log.Printf("history maintenance failed: %v", err)
					}
				}
			}
			select {
			case <-ctx.Done():
				return
			case <-ticker.C:
			}
		}
	}()
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.settingsMu.Lock()
	m.stopping = true
	if m.maintenanceCancel != nil {
		m.maintenanceCancel()
	}
	m.mu.Lock()
	for _, cancel := range m.cancels {
		cancel()
	}
	m.mu.Unlock()
	m.settingsMu.Unlock()
	done := make(chan struct{})
	go func() { m.workers.Wait(); close(done) }()
	select {
	case <-done:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}
