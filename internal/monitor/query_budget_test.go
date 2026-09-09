package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestCachedViewsAreIsolatedAndInvalidated(t *testing.T) {
	m, store, _, now := setup(t)
	ctx := context.Background()
	m.step(ctx, *now)
	first, err := m.WindowOverview(ctx, "7d")
	if err != nil {
		t.Fatal(err)
	}
	first.Plan.Nodes[0].Name = "changed"
	first.Rows[0].State.Status = "changed"
	second, err := m.WindowOverview(ctx, "7d")
	if err != nil || second.Plan.Nodes[0].Name == "changed" || second.Rows[0].State.Status == "changed" {
		t.Fatal("cache aliased caller", err)
	}
	if _, err = m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	paused, err := m.WindowOverview(ctx, "7d")
	if err != nil || paused.Plan.Enabled {
		t.Fatal("pause used stale cache", err)
	}
	store.Close()
	if _, err = m.WindowOverview(ctx, "7d"); err != nil {
		t.Fatal("cache hit queried database", err)
	}
	*now = now.Add(11 * time.Second)
	if _, err = m.WindowOverview(ctx, "7d"); err == nil {
		t.Fatal("cache did not expire")
	}
}

func TestHeavyQueryBudgetAndHealthyFailoverDoNotReadHistory(t *testing.T) {
	m, source, now := failoverFixture(t)
	release, err := m.acquireHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	if _, err = m.WindowOverview(context.Background(), "7d"); !errors.Is(err, ErrHistoryBusy) {
		t.Fatal("parallel heavy query admitted", err)
	}
	m.states["node-a"] = model.MonitorState{Status: "healthy", LastAt: *now, LastSuccess: *now}
	m.store.Close()
	m.failover(context.Background(), *now)
	if len(source.switches) != 0 || m.fault {
		t.Fatal("healthy current node entered history path")
	}
}

type foregroundSource struct{ *fakeSource }

func (*foregroundSource) MonitorForegroundBusy() bool { return true }
func TestMaintenanceYieldsToForegroundAndIdles(t *testing.T) {
	m, store, source, now := setup(t)
	if delay := m.maintenance(context.Background(), *now); delay != 30*time.Second {
		t.Fatal("empty maintenance does not back off", delay)
	}
	m.source = &foregroundSource{source}
	store.Close()
	if delay := m.maintenance(context.Background(), *now); delay != 30*time.Second || m.fault {
		t.Fatal("foreground scan did not defer maintenance")
	}
}

func TestHistorySlotDoesNotBlockConfirmedFailover(t *testing.T) {
	m, source, now := failoverFixture(t)
	release, err := m.acquireHistory(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer release()
	m.failover(context.Background(), *now)
	if len(source.switches) != 1 || source.switches[0] != "B" {
		t.Fatal("history budget blocked failover", source.switches)
	}
}
