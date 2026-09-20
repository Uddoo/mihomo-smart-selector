package monitor

import (
	"context"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestCachedRowMatchesFreshEvidenceAcrossClockBoundaries(t *testing.T) {
	m, store, _, _ := setup(t)
	ctx := context.Background()
	n := m.plan.Nodes[0]
	series := model.MonitorSeries{ID: n.SeriesID, Node: n, Anchor: n.Anchor}
	anchor := time.Unix(n.Anchor, 0).UTC()
	for i := 0; i < 750; i++ {
		if i%13 == 4 {
			continue
		} // genuine gaps
		outcome := []string{"failure", "failure", "unknown", "success", "success"}[i%5]
		if _, err := store.RecordMonitor(ctx, m.plan.ID, model.MonitorSample{NodeID: n.ID, Kind: "baseline", Slot: int64(i), At: anchor.Add(time.Duration(i*120+3) * time.Second), Outcome: outcome, DelayMS: 100 + i%30}, model.MonitorState{}, nil); err != nil {
			t.Fatal(err)
		}
	}
	for _, window := range []time.Duration{time.Hour, 24 * time.Hour, 7 * 24 * time.Hour} {
		for _, second := range []int{-1, 0, 1, 5, 119, 120, 121, 239, 240, 3599, 3600, 3601, 86399, 86400, 86401, 86520, 86521} {
			now := anchor.Add(time.Duration(second) * time.Second)
			cached, err := m.historyRow(ctx, series, now, window, true)
			if err != nil {
				t.Fatal(err)
			}
			fresh, err := m.historyRow(ctx, series, now, window, false)
			if err != nil || !reflect.DeepEqual(cached, fresh) {
				t.Fatalf("window %s second %d: cached=%+v fresh=%+v err=%v", window, second, cached.Metrics, fresh.Metrics, err)
			}
		}
	}
}

func TestRowCacheInvalidatesOnlyChangedSeriesAndCleanup(t *testing.T) {
	m, store, source, now := setup(t)
	ctx := context.Background()
	source.nodes = append(source.nodes, model.MonitorNode{ID: "node-b", Name: "B", Provider: "one", Protocol: "VLESS"})
	p, err := m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"A", "B"}})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(70 * time.Second)
	read := func(i int) model.MonitorRow {
		t.Helper()
		n := p.Nodes[i]
		r, e := m.historyRow(ctx, model.MonitorSeries{ID: n.SeriesID, Node: n, Anchor: n.Anchor}, *now, 24*time.Hour, true)
		if e != nil {
			t.Fatal(e)
		}
		return r
	}
	a, b := read(0), read(1)
	keyB := rowKey{p.Nodes[1].SeriesID, 24 * time.Hour}
	before := m.queries.rows[keyB].at
	write := func(kind string, slot int64) {
		t.Helper()
		_, e := store.RecordMonitor(ctx, p.ID, model.MonitorSample{NodeID: p.Nodes[0].ID, Kind: kind, Slot: slot, At: *now, Outcome: "success", DelayMS: 125}, model.MonitorState{}, nil)
		if e != nil {
			t.Fatal(e)
		}
	}
	write("current", 1)
	if got := read(0); got.EvidenceVersion != a.EvidenceVersion {
		t.Fatal("extra check invalidated baseline")
	}
	write("baseline", 0) // late arrival after both empty rows were cached
	*now = now.Add(time.Second)
	updated := read(0)
	if updated.Metrics.Samples != 1 || updated.EvidenceVersion == a.EvidenceVersion {
		t.Fatal("late baseline not visible", updated)
	}
	if got := read(1); got.EvidenceVersion != b.EvidenceVersion || !m.queries.rows[keyB].at.Equal(before) {
		t.Fatal("unrelated series recomputed")
	}
	updated.Series[0].DelayMS = 999
	if read(0).Series[0].DelayMS != 125 {
		t.Fatal("cache aliases response")
	}
	if err = store.CleanupMonitor(ctx, *now); err != nil {
		t.Fatal(err)
	}
	if read(1).EvidenceVersion == b.EvidenceVersion {
		t.Fatal("cleanup did not invalidate all rows")
	}
}

func TestCachedOverviewKeepsCurrentStateFresh(t *testing.T) {
	m, _, _, now := setup(t)
	ctx := context.Background()
	m.step(ctx, *now)
	if _, err := m.WindowOverview(ctx, "24h"); err != nil {
		t.Fatal(err)
	}
	key := rowKey{m.plan.Nodes[0].SeriesID, 24 * time.Hour}
	entry := m.queries.rows[key]
	// Stay inside the same window boundaries while only health changes.
	*now = now.Add(5 * time.Second)
	m.mu.Lock()
	m.states[m.plan.Nodes[0].ID] = model.MonitorState{Status: "unavailable", LastAt: *now, Failures: 2}
	m.dataVersion++
	m.mu.Unlock()
	o, err := m.WindowOverview(ctx, "24h")
	if err != nil || o.Rows[0].State.Status != "unavailable" || !m.queries.rows[key].at.Equal(entry.at) {
		t.Fatal("state refresh repeated history or stayed stale", o, err)
	}
}

func TestIdleTaskPreflightPreservesDueWork(t *testing.T) {
	m, _, _, now := setup(t)
	ctx := context.Background()
	if !m.ready(*now) {
		t.Fatal("new task skipped")
	}
	m.step(ctx, *now)
	m.correlate(ctx, *now)
	if m.ready(now.Add(time.Second)) {
		t.Fatal("idle task dispatched")
	}
	if err := m.Retest(m.plan.Nodes[0].ID, m.plan.Revision); err != nil {
		t.Fatal(err)
	}
	if !m.ready(now.Add(time.Second)) {
		t.Fatal("manual retest skipped")
	}
	delete(m.manual, m.plan.Nodes[0].ID)
	if !m.ready(now.Add(30 * time.Second)) {
		t.Fatal("metadata/current check skipped")
	}
	if _, err := m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	if m.ready(now.Add(time.Second)) {
		t.Fatal("paused task dispatched idle tick")
	}
	if !m.ready(now.Add(30 * time.Second)) {
		t.Fatal("paused correlation reconciliation skipped")
	}
}
