package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func BenchmarkSevenDayOverviewFromNinetyDayArchive(b *testing.B) {
	ctx := context.Background()
	path := filepath.Join(b.TempDir(), "bench.db")
	store, err := history.Open(path)
	if err != nil {
		b.Fatal(err)
	}
	defer store.Close()
	now := time.Now().UTC().Truncate(time.Hour)
	start := now.Add(-90 * 24 * time.Hour)
	f := &fakeSource{profile: config.ProbeProfile{ID: "chatgpt", Probes: []config.Probe{{Name: "test", URL: "https://example.com/"}}}, reachable: true}
	names := []string{}
	for i := 0; i < 6; i++ {
		name := fmt.Sprintf("N%d", i)
		names = append(names, name)
		f.nodes = append(f.nodes, model.MonitorNode{ID: name, Name: name, Provider: "P", Protocol: "VLESS"})
	}
	f.current = names[0]
	m, err := New(store, f)
	if err != nil {
		b.Fatal(err)
	}
	m.now = func() time.Time { return start }
	p, err := m.Save(ctx, model.MonitorRequest{Enabled: true, Group: "g", ProfileID: "chatgpt", Nodes: names})
	if err != nil {
		b.Fatal(err)
	}
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		b.Fatal(err)
	}
	defer raw.Close()
	tx, err := raw.Begin()
	if err != nil {
		b.Fatal(err)
	}
	stmt, err := tx.Prepare(`INSERT INTO monitor_hourly(series_id,hour,payload,updated_at) VALUES(?,?,?,?)`)
	if err != nil {
		b.Fatal(err)
	}
	for _, n := range p.Nodes {
		for hour := 0; hour < 90*24; hour++ {
			slots := [][5]int64{}
			for j := 0; j < 30; j++ {
				slot := int64(hour*30 + j)
				slots = append(slots, [5]int64{slot, 1, int64(150 + j), n.Anchor + slot*120, 1})
			}
			data, _ := json.Marshal(map[string]any{"slots": slots})
			if _, err = stmt.Exec(n.SeriesID, (n.Anchor+int64(hour*3600))/3600*3600, string(data), now.Unix()); err != nil {
				b.Fatal(err)
			}
		}
		m.states[n.ID] = model.MonitorState{Status: "healthy", LastAt: now, LastSuccess: now}
	}
	stmt.Close()
	if err = tx.Commit(); err != nil {
		b.Fatal(err)
	}
	m.now = func() time.Time { return now }
	m.current = names[0]
	m.observedAt = now
	b.Run("cold", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			o, err := m.overview(ctx, "7d", true)
			if err != nil || len(o.Rows) != 6 || o.Rows[0].Metrics.Samples < 5000 {
				b.Fatal(o, err)
			}
		}
	})
	if _, err = m.WindowOverview(ctx, "7d"); err != nil {
		b.Fatal(err)
	}
	b.Run("cached", func(b *testing.B) {
		b.ReportAllocs()
		for i := 0; i < b.N; i++ {
			o, err := m.WindowOverview(ctx, "7d")
			if err != nil || len(o.Rows) != 6 {
				b.Fatal(o, err)
			}
		}
	})
}
