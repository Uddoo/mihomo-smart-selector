package scan

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestControllerChangeCannotReuseScanOrReconcileForeignSwitch(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	store, err := history.Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{"work": {Name: "work", Type: "Selector", Now: "old", All: []string{"a"}}}}
	original := NewManager(cfg, fake, store)
	now := time.Now().UTC()
	s := model.Scan{ID: "original", ControllerScope: original.bindingScope(), Status: model.ScanComplete, StartedAt: now, CompletedAt: &now, Request: model.ScanRequest{TargetGroup: "work", ProfileID: "internet-baseline", Mode: "quick"}, Results: []model.NodeResult{{Name: "a", Rank: 1, SuccessRate: 1, MeasuredAt: now}}}
	if err := store.CreateScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	op, err := store.RecordSwitch(ctx, model.SwitchEvent{ControllerScope: original.bindingScope(), ScanID: s.ID, Group: "work", Selected: "a", Status: "unknown", RequestID: "original-switch", CreatedAt: now})
	if err != nil {
		t.Fatal(err)
	}
	cfg.Mihomo.Controller = "http://another-controller.example:9090"
	changed := NewManager(cfg, fake, store)
	if _, err := changed.SelectRequest(ctx, s.ID, "a", "new-request"); err == nil || !strings.Contains(err.Error(), "Controller") {
		t.Fatal("foreign scan selectable", err)
	}
	if _, err := changed.SelectRequest(ctx, s.ID, "a", op.RequestID); err == nil {
		t.Fatal("foreign retry accepted")
	}
	if _, err := changed.Retest(ctx, s.ID, "a"); err == nil {
		t.Fatal("foreign scan retested implicitly")
	}
	if _, err := changed.ReconcileSwitch(ctx, op.ID); err == nil {
		t.Fatal("foreign switch reconciled against the wrong Controller")
	}
	if fake.selected != "" {
		t.Fatal("Controller was changed")
	}
	decorated, err := changed.Get(ctx, s.ID)
	if err != nil || !strings.Contains(decorated.Results[0].SelectionReason, "Controller") {
		t.Fatal("UI did not receive the scope reason", err)
	}
	if blocked, err := store.UnresolvedSwitch(ctx, "work", changed.bindingScope()); err != nil || blocked {
		t.Fatal("foreign unknown switch blocked the new Controller", err)
	}
	if blocked, err := store.UnresolvedSwitch(ctx, "work", original.bindingScope()); err != nil || !blocked {
		t.Fatal("original pending evidence lost", err)
	}
	if _, err := original.ReconcileSwitch(ctx, op.ID); err != nil {
		t.Fatal("original Controller cannot reconcile", err)
	}
}
