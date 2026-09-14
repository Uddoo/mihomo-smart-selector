package history

import (
	"context"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestWarningsMigrationPreservesLegacyScan(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	store, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer func() { _ = store.Close() }()
	now := time.Now().UTC()
	s := model.Scan{ID: "legacy", Status: model.ScanComplete, StartedAt: now, CompletedAt: &now,
		Request: model.ScanRequest{TargetGroup: "work", Mode: "quick"}, Results: []model.NodeResult{{Rank: 1, Name: "node", VerifiedRegion: "JP"}}}
	if err := store.CreateScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	// Recreate the prior schema while retaining its existing scan and results.
	if _, err := store.db.ExecContext(ctx, `ALTER TABLE scans DROP COLUMN warnings_json`); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err := store.GetScan(ctx, s.ID)
	if err != nil || len(got.Warnings) != 0 || got.Status != s.Status || !reflect.DeepEqual(got.Results, s.Results) {
		t.Fatalf("legacy scan changed: %+v %v", got, err)
	}
	s.Warnings = []model.ScanWarning{{Code: "probe_restore_failed", Phase: "egress", Group: "probe", Message: "未能恢复探测组，请到 Controller 核对当前选择。"}}
	if err := store.CompleteScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = Open(path)
	if err != nil {
		t.Fatal(err)
	}
	got, err = store.GetScan(ctx, s.ID)
	if err != nil || !reflect.DeepEqual(got.Warnings, s.Warnings) || !reflect.DeepEqual(got.Results, s.Results) {
		t.Fatalf("warning changed evidence: %+v %v", got, err)
	}
	recent, err := store.RecentScans(ctx, 10)
	if err != nil || len(recent) != 1 || !reflect.DeepEqual(recent[0].Warnings, s.Warnings) {
		t.Fatalf("recent history lost warning: %+v %v", recent, err)
	}
}
