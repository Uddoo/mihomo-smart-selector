package history

import (
	"context"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestRestartRecoveryPreservesEvidenceAndMarksPendingUnknown(t *testing.T) {
	ctx := context.Background()
	path := t.TempDir() + "/db"
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now()
	s := model.Scan{ID: "running", Status: model.ScanRunning, StartedAt: now, Progress: model.ScanProgress{Completed: 1, Total: 2}}
	if err = store.CreateScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err = store.CompleteScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	op, err := store.RecordSwitch(ctx, model.SwitchEvent{ScanID: s.ID, Status: "pending", RequestID: "key", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	store.Close()
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	if err = store.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
	loaded, err := store.GetScan(ctx, s.ID)
	if err != nil || loaded.Status != model.ScanInterrupted || loaded.Progress.Completed != 1 {
		t.Fatalf("recovered=%+v err=%v", loaded, err)
	}
	audit, err := store.SwitchByID(ctx, op.ID)
	if err != nil || audit.Status != "unknown" {
		t.Fatalf("audit=%+v err=%v", audit, err)
	}
	recent, err := store.RecentScans(ctx, 20)
	if err != nil || len(recent) != 1 || recent[0].Results != nil {
		t.Fatalf("recent=%+v err=%v", recent, err)
	}
	if err = store.RecoverInterrupted(ctx); err != nil {
		t.Fatal(err)
	}
}

func TestRetentionPreservesRunningAndUnresolvedAndCascadesResults(t *testing.T) {
	ctx := context.Background()
	store, err := Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	old := time.Now().AddDate(0, 0, -400)
	for _, id := range []string{"old", "protected", "running", "recent"} {
		s := model.Scan{ID: id, Status: model.ScanComplete, StartedAt: old, CompletedAt: &old, Results: []model.NodeResult{{Name: "a", Rank: 1}}}
		if id == "running" {
			s.Status = model.ScanRunning
		}
		if id == "recent" {
			s.StartedAt = time.Now()
		}
		if err = store.CreateScan(ctx, s); err != nil {
			t.Fatal(err)
		}
		if err = store.CompleteScan(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	pending, err := store.RecordSwitch(ctx, model.SwitchEvent{ScanID: "protected", Status: "unknown", CreatedAt: old})
	if err != nil {
		t.Fatal(err)
	}
	_, err = store.RecordSwitch(ctx, model.SwitchEvent{ScanID: "old", Status: "confirmed", CreatedAt: old})
	if err != nil {
		t.Fatal(err)
	}
	result, err := store.Cleanup(ctx, config.RetentionPolicy{ScanDays: 30, MaxScans: 1, AuditDays: 180, MaxAudit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if result.Scans != 1 || result.Audit != 1 {
		t.Fatalf("cleanup=%+v", result)
	}
	for _, id := range []string{"protected", "running", "recent"} {
		if _, err = store.GetScan(ctx, id); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = store.SwitchByID(ctx, pending.ID); err != nil {
		t.Fatal(err)
	}
	var orphans int
	err = store.db.QueryRow(`SELECT COUNT(*) FROM scan_results WHERE scan_id='old'`).Scan(&orphans)
	if err != nil || orphans != 0 {
		t.Fatalf("orphan results=%d err=%v", orphans, err)
	}
	if result.Stats.DatabaseBytes == 0 || result.Stats.Unresolved != 1 {
		t.Fatalf("stats=%+v", result.Stats)
	}
}
