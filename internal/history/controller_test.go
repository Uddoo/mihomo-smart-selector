package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestLegacyControllerMigrationPreservesKnownScopes(t *testing.T) {
	ctx := context.Background()
	store, err := Open(filepath.Join(t.TempDir(), "history.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	for _, s := range []model.Scan{{ID: "legacy", StartedAt: time.Now()}, {ID: "scoped", ControllerScope: "http://known.example", StartedAt: time.Now()}} {
		if err := store.CreateScan(ctx, s); err != nil {
			t.Fatal(err)
		}
	}
	op, err := store.RecordSwitch(ctx, model.SwitchEvent{ScanID: "legacy", CreatedAt: time.Now(), Status: "unknown"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.BindLegacyController(ctx, "http://original.example/"); err != nil {
		t.Fatal(err)
	}
	if err := store.BindLegacyController(ctx, "http://different.example"); err != nil {
		t.Fatal(err)
	}
	legacy, err := store.GetScan(ctx, "legacy")
	if err != nil || legacy.ControllerScope != "http://original.example" {
		t.Fatal("legacy history moved", err)
	}
	scoped, err := store.GetScan(ctx, "scoped")
	if err != nil || scoped.ControllerScope != "http://known.example" {
		t.Fatal("known history moved", err)
	}
	event, err := store.SwitchByID(ctx, op.ID)
	if err != nil || event.ControllerScope != legacy.ControllerScope || event.Status != "unknown" {
		t.Fatal("pending evidence lost", err)
	}
}
