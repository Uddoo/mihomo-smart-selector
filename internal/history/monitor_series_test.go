package history

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestLegacyMonitorMigrationPreservesPlanAndSlots(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "legacy.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	p := model.MonitorPlan{ID: "legacy", Revision: 7, Enabled: true, AutoSwitch: true, Group: "g", ProfileID: "chatgpt", ProfileHash: "hash", CreatedAt: start, Nodes: []model.MonitorNode{{ID: "a", Name: "A", Provider: "P", Protocol: "VLESS"}}}
	payload, _ := json.Marshal(p)
	if _, err = s.db.Exec(`INSERT INTO monitor_plans VALUES(?,?,?)`, "scope", 7, string(payload)); err != nil {
		t.Fatal(err)
	}
	v := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 3, At: start.Add(365 * time.Second), Outcome: "failure"}
	payload, _ = json.Marshal(v)
	if _, err = s.db.Exec(`INSERT INTO monitor_samples VALUES(?,?,?,?,?,?)`, p.ID, "a", "baseline", 3, v.At.Unix(), string(payload)); err != nil {
		t.Fatal(err)
	}
	if _, err = s.db.Exec(`INSERT INTO monitor_samples VALUES(?,?,?,?,?,?)`, "orphan", "a", "baseline", 3, v.At.Unix(), string(payload)); err != nil {
		t.Fatal(err)
	}
	state := model.MonitorState{Status: "unavailable", LastAt: v.At, Failures: 2}
	payload, _ = json.Marshal(state)
	s.db.Exec(`INSERT INTO monitor_states VALUES(?,?,?)`, p.ID, "a", string(payload))
	s.Close()
	for i := 0; i < 2; i++ {
		s, err = Open(path)
		if err != nil {
			t.Fatal(err)
		}
		restored, e := s.MonitorPlan(ctx, "scope")
		if e != nil || restored.ID != p.ID || restored.Revision != 7 || !restored.AutoSwitch || restored.Nodes[0].Anchor != start.Unix() {
			t.Fatal(restored, e)
		}
		items, e := s.MonitorSamples(ctx, p.ID, start)
		if e != nil || len(items) != 1 || items[0].ScheduledAt != start.Unix()+360 || items[0].Outcome != "failure" {
			t.Fatal(items, e)
		}
		states, e := s.MonitorStates(ctx, p.ID)
		if e != nil || states["a"].Failures != 2 {
			t.Fatal(states, e)
		}
		var count int
		s.db.QueryRow(`SELECT COUNT(*) FROM monitor_observations`).Scan(&count)
		if count != 1 {
			t.Fatal("orphan guessed or import repeated", count)
		}
		s.Close()
	}
}

func TestHourlyRollupExactBoundariesAndLateSamples(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	start := time.Date(2026, 9, 1, 0, 0, 20, 0, time.UTC)
	p := model.MonitorPlan{ID: "p", Revision: 1, ProfileID: "chatgpt", ProfileHash: "h", CreatedAt: start, Nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	if err = s.SaveMonitorPlan(ctx, "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	saved, _ := s.MonitorPlan(ctx, "scope")
	series, e := s.SeriesDefinition(ctx, "scope", saved.Nodes[0].SeriesID)
	if e != nil {
		t.Fatal(e)
	}
	for i := 0; i < 90; i++ {
		if i == 44 {
			continue
		}
		outcome := "success"
		if i == 29 || i == 30 || i == 32 {
			outcome = "failure"
		}
		if i == 31 {
			outcome = "unknown"
		}
		v := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: int64(i), At: start.Add(time.Duration(i*120+5) * time.Second), Outcome: outcome, DelayMS: 100 + i}
		if _, err = s.RecordMonitor(ctx, p.ID, v, model.MonitorState{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	from, to := start.Add(29*120*time.Second), start.Add(88*120*time.Second)
	raw, err := s.SeriesSamples(ctx, series, from, to, false)
	if err != nil {
		t.Fatal(err)
	}
	if err = s.AggregateMonitor(ctx, start.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	if err = s.AggregateMonitor(ctx, start.Add(5*time.Hour)); err != nil {
		t.Fatal(err)
	}
	var count int
	s.db.QueryRow(`SELECT COUNT(*) FROM monitor_hourly`).Scan(&count)
	if count != 3 {
		t.Fatal("unexpected hours", count)
	}
	// Delete sealed raw data as retention would; compact slots remain sufficient.
	s.db.Exec(`DELETE FROM monitor_observations`)
	agg, err := s.SeriesSamples(ctx, series, from, to, false)
	if err != nil || len(raw) != len(agg) {
		t.Fatal(len(raw), len(agg), err)
	}
	for i := range raw {
		a, b := raw[i], agg[i]
		if a.Slot != b.Slot || a.Outcome != b.Outcome || a.DelayMS != b.DelayMS || a.ScheduledAt != b.ScheduledAt {
			t.Fatalf("changed slot: %+v %+v", a, b)
		}
	}
	late := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 44, At: start.Add(6 * time.Hour), Outcome: "success", DelayMS: 456}
	if _, err = s.RecordMonitor(ctx, p.ID, late, model.MonitorState{}, nil); err != nil {
		t.Fatal(err)
	}
	if err = s.AggregateMonitor(ctx, start.Add(7*time.Hour)); err != nil {
		t.Fatal(err)
	}
	agg, err = s.SeriesSamples(ctx, series, from, to, false)
	if err != nil || len(agg) != len(raw)+1 {
		t.Fatal("late sample discarded prior compact slots", len(agg), err)
	}
}

func TestCleanupDoesNotDeleteUnaggregatedEvidence(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC()
	start := now.Add(-8 * 24 * time.Hour)
	p := model.MonitorPlan{ID: "p", Revision: 1, ProfileID: "chatgpt", CreatedAt: start, Nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	s.SaveMonitorPlan(ctx, "scope", p, 0)
	s.RecordMonitor(ctx, p.ID, model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 0, At: start, Outcome: "failure"}, model.MonitorState{}, nil)
	if err = s.CleanupMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	var n int
	s.db.QueryRow(`SELECT COUNT(*) FROM monitor_observations`).Scan(&n)
	if n != 1 {
		t.Fatal("deleted only evidence")
	}
	if err = s.AggregateMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	if err = s.CleanupMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM monitor_observations`).Scan(&n)
	if n != 0 {
		t.Fatal("sealed raw not cleaned")
	}
	s.db.QueryRow(`SELECT COUNT(*) FROM monitor_hourly`).Scan(&n)
	if n != 1 {
		t.Fatal("lost aggregate")
	}
}
