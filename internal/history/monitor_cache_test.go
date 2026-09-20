package history

import (
	"context"
	"encoding/json"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestTaskSnapshotIsolationAndExternalRevision(t *testing.T) {
	path := filepath.Join(t.TempDir(), "cache.db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	p := model.MonitorPlan{ID: "plan", Revision: 1, Group: "g", CreatedAt: time.Now().UTC(), Nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	if err = s.SaveMonitorPlan(ctx, "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	first, err := s.MonitorTasks(ctx, "scope")
	if err != nil {
		t.Fatal(err)
	}
	first[0].Plan.Nodes[0].Name = "mutated"
	second, err := s.MonitorTasks(ctx, "scope")
	if err != nil || second[0].Plan.Nodes[0].Name != "A" {
		t.Fatal(second, err)
	}
	other, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer other.Close()
	p = second[0].Plan
	p.Revision++
	p.Enabled = true
	if err = other.SaveMonitorTask(ctx, "scope", p, 1); err != nil {
		t.Fatal(err)
	}
	third, err := s.MonitorTasks(ctx, "scope")
	if err != nil || third[0].Plan.Revision != 2 || !third[0].Plan.Enabled {
		t.Fatal("external write hidden by cache", third, err)
	}
}

func TestCompactArchiveMergePreservesRawPrecedenceAndExtraOrdering(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "merge.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	start := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC)
	p := model.MonitorPlan{ID: "p", Revision: 1, CreatedAt: start, Nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	if err = s.SaveMonitorPlan(ctx, "scope", p, 0); err != nil {
		t.Fatal(err)
	}
	pSaved, err := s.MonitorPlan(ctx, "scope")
	if err != nil {
		t.Fatal(err)
	}
	series, err := s.SeriesDefinition(ctx, "scope", pSaved.Nodes[0].SeriesID)
	if err != nil {
		t.Fatal(err)
	}
	data, _ := json.Marshal(hourPayload{Slots: [][5]int64{{0, 2, 200, start.Unix(), 2}, {1, 1, 300, start.Unix() + 120, 1}}})
	if _, err = s.db.Exec(`INSERT INTO monitor_hourly(series_id,hour,payload,updated_at) VALUES(?,?,?,?)`, series.ID, start.Unix(), string(data), start.Unix()); err != nil {
		t.Fatal(err)
	}
	for i, kind := range []string{"baseline", "confirmation", "current", "manual"} {
		second := []int{0, 0, 60, 120}[i]
		if _, err = s.RecordMonitor(ctx, p.ID, model.MonitorSample{NodeID: "a", Kind: kind, Slot: 0, At: start.Add(time.Duration(second) * time.Second), Outcome: "success", DelayMS: 100}, model.MonitorState{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	got, err := s.SeriesSamples(ctx, series, start, start.Add(2*time.Minute), true)
	if err != nil || len(got) != 5 {
		t.Fatal(got, err)
	}
	for i, kind := range []string{"baseline", "confirmation", "current", "baseline", "manual"} {
		if got[i].Kind != kind {
			t.Fatalf("item %d: %+v", i, got)
		}
	}
	if got[0].Outcome != "success" || got[0].DelayMS != 100 || got[0].Resolution != "raw" || got[3].DelayMS != 300 || got[3].Resolution != "hourly" {
		t.Fatal("raw/archive evidence changed", got)
	}
}
