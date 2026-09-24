package history

import (
	"context"
	"encoding/json"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestCompactBaselineMatchesFullHistory(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "baseline.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	anchor := time.Date(2026, 9, 1, 0, 0, 17, 0, time.UTC).Unix()
	series := model.MonitorSeries{ID: "series", Anchor: anchor, Node: model.MonitorNode{ID: "node"}}
	hours := map[int64]*hourPayload{}
	for i := int64(0); i < 210; i++ {
		if i%11 == 0 {
			continue
		}
		scheduled := anchor + i*120
		hour := scheduled / 3600 * 3600
		if hours[hour] == nil {
			hours[hour] = &hourPayload{}
		}
		outcome := int64(1)
		if i%7 == 0 {
			outcome = 2
		}
		if i%17 == 0 {
			outcome = 0
		}
		hours[hour].Slots = append(hours[hour].Slots, [5]int64{i, outcome, i * 13 % 2000, scheduled + 3, i % 12})
	}
	for hour, payload := range hours {
		data, _ := json.Marshal(payload)
		if _, err := s.db.Exec(`INSERT INTO monitor_hourly(series_id,hour,payload,updated_at) VALUES(?,?,?,?)`, series.ID, hour, string(data), anchor); err != nil {
			t.Fatal(err)
		}
	}
	// Overlapping raw records win. SQL schedules are intentionally unrelated to
	// slot ordering, and two out-of-window slots exercise the sparse fallback.
	for i := int64(-1); i <= 210; i++ {
		if i%3 == 0 {
			continue
		}
		scheduled := anchor + ((i+211)*17%210)*120
		v := model.MonitorSample{NodeID: "node", SeriesID: series.ID, Kind: "baseline", Slot: i, ScheduledAt: scheduled, At: time.Unix(scheduled+4, 0).UTC(), Outcome: "success", DelayMS: int(i + 1), Reason: "raw detail", ReasonCode: "ok", DataVersion: 2}
		if i%5 == 0 {
			v.Outcome = "failure"
		}
		if i%13 == 0 {
			v.Outcome = "unknown"
		}
		data, _ := json.Marshal(v)
		if _, err := s.db.Exec(`INSERT INTO monitor_observations(series_id,kind,slot,at,scheduled_at,payload) VALUES(?,?,?,?,?,?)`, series.ID, v.Kind, v.Slot, v.At.Unix(), v.ScheduledAt, string(data)); err != nil {
			t.Fatal(err)
		}
	}
	type point struct {
		Slot    int64
		Outcome string
		Delay   int
	}
	for _, interval := range []struct{ from, to int64 }{{anchor - 7*86400, anchor + 211*120}, {anchor + 3000, anchor + 18000}, {anchor + 205*120, anchor + 220*120}, {anchor - 7200, anchor - 1}, {anchor + 300*120, anchor + 301*120}} {
		from, to := time.Unix(interval.from, 0), time.Unix(interval.to, 0)
		full, err := s.SeriesSamples(ctx, series, from, to, false)
		if err != nil {
			t.Fatal(err)
		}
		want, got := []point{}, []point{}
		for _, sample := range full {
			want = append(want, point{sample.Slot, sample.Outcome, sample.DelayMS})
		}
		tail, err := s.VisitSeriesBaseline(ctx, series, from, to, func(slot int64, outcome string, delay int) { got = append(got, point{slot, outcome, delay}) })
		if err != nil {
			t.Fatal(err)
		}
		if !reflect.DeepEqual(want, got) {
			t.Fatalf("baseline changed for %+v: want=%v got=%v", interval, want, got)
		}
		if !reflect.DeepEqual(full[max(0, len(full)-60):], tail) {
			t.Fatalf("recent evidence changed for %+v", interval)
		}
	}
	cancelled, cancel := context.WithCancel(ctx)
	cancel()
	if _, err := s.VisitSeriesBaseline(cancelled, series, time.Unix(anchor, 0), time.Unix(anchor+86400, 0), func(int64, string, int) {}); err == nil {
		t.Fatal("cancelled read succeeded")
	}
}
