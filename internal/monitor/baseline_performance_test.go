package monitor

import (
	"context"
	"database/sql"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Both readers use the same database and time bounds; the full reader remains
// the independent evidence oracle for raw, overlapping and archived histories.
func BenchmarkBaselineReaders(b *testing.B) {
	for _, source := range []string{"raw", "mixed", "archive"} {
		b.Run(source, func(b *testing.B) {
			ctx := context.Background()
			path := filepath.Join(b.TempDir(), "baseline.db")
			store, err := history.Open(path)
			if err != nil {
				b.Fatal(err)
			}
			defer store.Close()
			db, err := sql.Open("sqlite", path)
			if err != nil {
				b.Fatal(err)
			}
			defer db.Close()
			tx, err := db.Begin()
			if err != nil {
				b.Fatal(err)
			}
			hourStmt, err := tx.Prepare(`INSERT INTO monitor_hourly(series_id,hour,payload,updated_at) VALUES(?,?,?,?)`)
			if err != nil {
				b.Fatal(err)
			}
			rawStmt, err := tx.Prepare(`INSERT INTO monitor_observations(series_id,kind,slot,at,scheduled_at,payload) VALUES(?,?,?,?,?,?)`)
			if err != nil {
				b.Fatal(err)
			}
			anchor := time.Date(2026, 9, 1, 0, 0, 0, 0, time.UTC).Unix()
			series := model.MonitorSeries{ID: "series", Anchor: anchor, Node: model.MonitorNode{ID: "node"}}
			now := time.Unix(anchor+7*86400, 0).UTC()
			for hour := 0; hour < 7*24; hour++ {
				slots := make([][5]int64, 0, 30)
				for j := 0; j < 30; j++ {
					slot := int64(hour*30 + j)
					scheduled := anchor + slot*120
					slots = append(slots, [5]int64{slot, 1, 150 + int64(j), scheduled + 2, 1})
					if source == "raw" || (source == "mixed" && hour >= 7*12) {
						v := model.MonitorSample{NodeID: "node", SeriesID: series.ID, Kind: "baseline", Slot: slot, ScheduledAt: scheduled, At: time.Unix(scheduled+4, 0).UTC(), Outcome: "success", DelayMS: 100 + j, ReasonCode: "ok", DataVersion: 2}
						if slot%19 == 0 {
							v.Outcome = "failure"
						}
						if slot%31 == 0 {
							v.Outcome = "unknown"
						}
						data, _ := json.Marshal(v)
						if _, err := rawStmt.Exec(series.ID, "baseline", slot, v.At.Unix(), scheduled, string(data)); err != nil {
							b.Fatal(err)
						}
					}
				}
				if source != "raw" {
					data, _ := json.Marshal(map[string]any{"slots": slots})
					if _, err := hourStmt.Exec(series.ID, anchor+int64(hour*3600), string(data), now.Unix()); err != nil {
						b.Fatal(err)
					}
				}
			}
			hourStmt.Close()
			rawStmt.Close()
			if err := tx.Commit(); err != nil {
				b.Fatal(err)
			}
			legacy := func() (model.MonitorMetrics, []model.MonitorSample, error) {
				samples, err := store.SeriesSamples(ctx, series, time.Unix(anchor, 0), now, false)
				if err != nil {
					return model.MonitorMetrics{}, nil, err
				}
				return windowMetrics(samples, anchor, now, 7*24*time.Hour), append([]model.MonitorSample{}, samples[max(0, len(samples)-60):]...), nil
			}
			compact := func() (model.MonitorMetrics, []model.MonitorSample, error) {
				a := newWindowAccumulator(anchor, now, 7*24*time.Hour, 5041)
				tail, err := store.VisitSeriesBaseline(ctx, series, time.Unix(anchor, 0), now, a.add)
				return a.finish(), tail, err
			}
			want, tail, err := legacy()
			if err != nil {
				b.Fatal(err)
			}
			got, recent, err := compact()
			if err != nil || !reflect.DeepEqual(want, got) || !reflect.DeepEqual(tail, recent) {
				b.Fatal("reader evidence differs", err, want, got)
			}
			for _, reader := range []struct {
				name string
				read func() (model.MonitorMetrics, []model.MonitorSample, error)
			}{{"full", legacy}, {"compact", compact}} {
				b.Run(reader.name, func(b *testing.B) {
					b.ReportAllocs()
					for i := 0; i < b.N; i++ {
						metrics, tail, err := reader.read()
						if err != nil || metrics.Expected != 5041 || len(tail) != 60 {
							b.Fatal(metrics, err)
						}
					}
				})
			}
		})
	}
}
