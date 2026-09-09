package history

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestCorrelationNeedsNewEvidenceAndDeduplicatesRecovery(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	o := model.MonitorCorrelation{PlanID: "p", Provider: "provider", Failed: 3, Comparable: 3, Monitored: 3, Reliable: true, Signature: "1", UpdatedAt: now}
	observe := func(signature string) {
		t.Helper()
		o.Signature = signature
		o.UpdatedAt = o.UpdatedAt.Add(30 * time.Second)
		if err = s.ObserveMonitorCorrelation(ctx, "scope", o); err != nil {
			t.Fatal(err)
		}
	}
	observe("1")
	observe("1")
	items, _ := s.MonitorCorrelations(ctx, "scope")
	if len(items) != 0 {
		t.Fatal("same evidence confirmed incident")
	}
	observe("2")
	observe("3")
	items, _ = s.MonitorCorrelations(ctx, "scope")
	if len(items) != 1 || items[0].Status != "active" {
		t.Fatal(items)
	}
	o.Reliable = false
	observe("4")
	items, _ = s.MonitorCorrelations(ctx, "scope")
	if items[0].Status != "uncertain" {
		t.Fatal("unknown declared recovery")
	}
	o.Reliable = true
	o.Failed = 0
	observe("5")
	observe("6")
	items, _ = s.MonitorCorrelations(ctx, "scope")
	if len(items) != 1 || items[0].Status != "recovered" {
		t.Fatal(items)
	}
	o.Failed = 3
	observe("7")
	observe("8")
	if err = s.EndMonitorCorrelations(ctx, "scope", "p", true, o.UpdatedAt); err != nil {
		t.Fatal(err)
	}
	observe("9")
	observe("10")
	items, _ = s.MonitorCorrelations(ctx, "scope")
	if len(items) != 3 {
		t.Fatal("closed episode was reopened", items)
	}
}

func TestActivityPaginationAcrossSourcesIsStable(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	at := time.Now().UTC().Truncate(time.Second)
	p := model.MonitorPlan{ID: "p", Revision: 1, Group: "g", CreatedAt: at, Nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	if err = s.SaveMonitorPlan(ctx, "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 12; i++ {
		v := model.MonitorSample{NodeID: "a", Kind: "manual", Slot: int64(i), At: at, Outcome: "success"}
		event := &model.MonitorEvent{NodeID: "a", NodeName: "A", At: at, Status: "healthy"}
		if _, err = s.RecordMonitor(ctx, "p", v, model.MonitorState{}, event); err != nil {
			t.Fatal(err)
		}
		if err = s.RecordMonitorSystem(ctx, "scope", "p", "unknown", "fixture", at); err != nil {
			t.Fatal(err)
		}
	}
	for i := 0; i < 3; i++ {
		if _, err = s.RecordSwitch(ctx, model.SwitchEvent{Group: "g", Status: "confirmed", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
	}
	o := model.MonitorCorrelation{PlanID: "p", Provider: "one", Failed: 3, Comparable: 3, Monitored: 3, Reliable: true, Nodes: []string{"A", "B", "C"}, UpdatedAt: at, Signature: "first"}
	if err = s.ObserveMonitorCorrelation(ctx, "scope", o); err != nil {
		t.Fatal(err)
	}
	o.Signature = "second"
	if err = s.ObserveMonitorCorrelation(ctx, "scope", o); err != nil {
		t.Fatal(err)
	}
	cursor := ""
	seen := map[string]bool{}
	last := ""
	for count := 0; count < 20; count++ {
		page, e := s.MonitorActivities(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), cursor, 7)
		if e != nil {
			t.Fatal(e)
		}
		for _, item := range page.Items {
			if seen[item.Key] {
				t.Fatal("duplicate event", item.Key)
			}
			if last != "" && item.Key > last {
				t.Fatal("cursor ordering changed")
			}
			seen[item.Key] = true
			last = item.Key
		}
		cursor = page.NextCursor
		if cursor == "" {
			break
		}
	}
	if len(seen) != 29 {
		t.Fatal("pagination lost events", len(seen))
	}
	if _, err = s.MonitorActivities(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), "invalid", 7); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}

func TestRetentionRevisionAndScopedCounts(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	p, err := s.MonitorRetention(ctx, "scope")
	if err != nil || p.RawDays != 7 || p.AggregateDays != 90 {
		t.Fatal(p, err)
	}
	p.RawDays = 2
	saved, err := s.SaveMonitorRetention(ctx, "scope", p)
	if err != nil || saved.Revision != 1 {
		t.Fatal(saved, err)
	}
	if _, err = s.SaveMonitorRetention(ctx, "scope", p); err == nil {
		t.Fatal("stale revision accepted")
	}
	saved.RawDays = 31
	if _, err = s.SaveMonitorRetention(ctx, "scope", saved); err == nil {
		t.Fatal("invalid policy accepted")
	}
	stats, err := s.MonitorStorage(ctx, "scope")
	if err != nil || stats.Policy.RawDays != 2 || stats.RawSamples != 0 || stats.DatabaseBytes <= 0 {
		t.Fatal(stats, err)
	}
}
