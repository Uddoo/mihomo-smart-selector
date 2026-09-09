package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"math/rand"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestSevenDayRawAndCompactedMetricsAreIdentical(t *testing.T) {
	ctx := context.Background()
	path := filepath.Join(t.TempDir(), "db")
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	f := &fakeSource{profile: config.ProbeProfile{ID: "chatgpt", Probes: []config.Probe{{Name: "trace", URL: "https://example.com/"}}}, nodes: []model.MonitorNode{{ID: "a", Name: "A"}}}
	m, err := New(store, f)
	if err != nil {
		t.Fatal(err)
	}
	start := time.Date(2026, 8, 1, 0, 0, 20, 0, time.UTC)
	now := start.Add(7 * 24 * time.Hour)
	m.now = func() time.Time { return start }
	p, err := m.Save(ctx, model.MonitorRequest{Enabled: true, Group: "g", ProfileID: "chatgpt", Nodes: []string{"A"}})
	if err != nil {
		t.Fatal(err)
	}
	node := p.Nodes[0]
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	tx, err := raw.Begin()
	if err != nil {
		t.Fatal(err)
	}
	stmt, _ := tx.Prepare(`INSERT INTO monitor_observations(series_id,kind,slot,at,scheduled_at,payload) VALUES(?,?,?,?,?,?)`)
	dirty, _ := tx.Prepare(`INSERT OR IGNORE INTO monitor_dirty_hours(series_id,hour) VALUES(?,?)`)
	random := rand.New(rand.NewSource(42))
	for i := 0; i <= 5040; i++ {
		if i%67 == 0 {
			continue
		}
		outcome := "success"
		if random.Intn(7) == 0 {
			outcome = "failure"
		}
		if i%97 == 0 {
			outcome = "unknown"
		}
		scheduled := node.Anchor + int64(i*120)
		v := model.MonitorSample{SeriesID: node.SeriesID, NodeID: node.ID, Kind: "baseline", Slot: int64(i), At: time.Unix(min(scheduled+3, now.Unix()), 0).UTC(), ScheduledAt: scheduled, Outcome: outcome, DelayMS: 50 + random.Intn(2500), ReasonCode: "node_path", DataVersion: 2}
		data, _ := json.Marshal(v)
		if _, err = stmt.Exec(node.SeriesID, "baseline", i, v.At.Unix(), scheduled, string(data)); err != nil {
			t.Fatal(err)
		}
		if _, err = dirty.Exec(node.SeriesID, scheduled/3600*3600); err != nil {
			t.Fatal(err)
		}
	}
	stmt.Close()
	dirty.Close()
	if err = tx.Commit(); err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return now }
	before, err := m.overview(ctx, "7d", true)
	if err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 10; i++ {
		if err = store.AggregateMonitor(ctx, now.Add(3*time.Hour)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = raw.Exec(`DELETE FROM monitor_observations`); err != nil {
		t.Fatal(err)
	}
	after, err := m.overview(ctx, "7d", true)
	if err != nil {
		t.Fatal(err)
	}
	if !reflect.DeepEqual(before.Rows[0].Metrics, after.Rows[0].Metrics) {
		t.Fatalf("aggregation changed metrics: before=%+v after=%+v", before.Rows[0].Metrics, after.Rows[0].Metrics)
	}
}
