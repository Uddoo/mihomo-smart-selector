package history

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestMonitorCleanupUsesBoundedBatches(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	p := model.MonitorPlan{ID: "p", Revision: 1, CreatedAt: now.Add(-8 * 24 * time.Hour), Nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	if err = s.SaveMonitorPlan(ctx, "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.MonitorPlan(ctx, "scope")
	sid := saved.Nodes[0].SeriesID
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 1200; i++ {
		if _, err = insertObservation(ctx, tx, model.MonitorSample{SeriesID: sid, NodeID: "a", Kind: "current", Slot: int64(i), At: p.CreatedAt, ScheduledAt: p.CreatedAt.Unix(), Outcome: "success"}); err != nil {
			t.Fatal(err)
		}
	}
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupMonitor(ctx, now); !errors.Is(err, ErrMonitorCleanupPending) {
		t.Fatal("large cleanup did not yield", err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM monitor_observations`).Scan(&n)
	if n != 700 {
		t.Fatal("unbounded deletion", n)
	}
	for i := 0; i < 4; i++ {
		err = s.CleanupMonitor(ctx, now)
		if err == nil {
			break
		}
		if !errors.Is(err, ErrMonitorCleanupPending) {
			t.Fatal(err)
		}
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM monitor_observations`).Scan(&n)
	if n != 0 {
		t.Fatal("cleanup failed to progress", n)
	}
}
