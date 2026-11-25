package history

import (
	"context"
	"errors"
	"path/filepath"
	"strconv"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func retentionSeries(t *testing.T, s *Store, scope string, start time.Time) model.MonitorSeries {
	t.Helper()
	return retentionPlanSeries(t, s, scope, start, 1)[0]
}

func retentionPlanSeries(t *testing.T, s *Store, scope string, start time.Time, n int) []model.MonitorSeries {
	t.Helper()
	ctx := context.Background()
	plan := model.MonitorPlan{ID: scope, Revision: 1, CreatedAt: start}
	for i := 0; i < n; i++ {
		plan.Nodes = append(plan.Nodes, model.MonitorNode{ID: strconv.Itoa(i), Name: strconv.Itoa(i)})
	}
	if err := s.SaveMonitorPlan(ctx, scope, plan, 0); err != nil {
		t.Fatal(err)
	}
	series, err := s.MonitorSeries(ctx, scope)
	if err != nil || len(series) != n {
		t.Fatalf("series for %s: %d, %v", scope, len(series), err)
	}
	return series
}

func seedRetentionSlots(t *testing.T, s *Store, series model.MonitorSeries, first, step, count int) {
	t.Helper()
	ctx := context.Background()
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		t.Fatal(err)
	}
	defer tx.Rollback()
	for i := 0; i < count; i++ {
		slot := int64(first + i*step)
		at := series.Anchor + slot*120
		sample := model.MonitorSample{SeriesID: series.ID, NodeID: "a", Kind: "baseline", Slot: slot, At: time.Unix(at, 0).UTC(), ScheduledAt: at, Outcome: "success", DelayMS: i + 1}
		if inserted, err := insertObservation(ctx, tx, sample); err != nil || !inserted {
			t.Fatalf("insert slot %d: %t, %v", slot, inserted, err)
		}
	}
	if err := tx.Commit(); err != nil {
		t.Fatal(err)
	}
}

func retentionStorage(t *testing.T, s *Store, scope string) model.MonitorStorage {
	t.Helper()
	stats, err := s.MonitorStorage(context.Background(), scope)
	if err != nil {
		t.Fatal(err)
	}
	return stats
}

func TestMonitorCleanupBaselineBatches(t *testing.T) {
	for _, tc := range []struct {
		name       string
		ageDays    int
		newerHours int
	}{{"expiry", 100, 0}, {"MaxHourly", 60, 1000}} {
		t.Run(tc.name, func(t *testing.T) {
			s, err := Open(filepath.Join(t.TempDir(), "db"))
			if err != nil {
				t.Fatal(err)
			}
			defer s.Close()
			ctx := context.Background()
			now := time.Now().UTC().Truncate(time.Hour)
			series := retentionSeries(t, s, "scope", now.AddDate(0, 0, -tc.ageDays))
			seedRetentionSlots(t, s, series, 0, 1, 600)
			if n, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil || n != 20 {
				t.Fatalf("initial rollup: %d hours, %v", n, err)
			}
			if tc.newerHours > 0 {
				policy := defaultMonitorRetention()
				policy.MaxHourly = tc.newerHours
				if _, err := s.SaveMonitorRetention(ctx, "scope", policy); err != nil {
					t.Fatal(err)
				}
				_, err := s.db.Exec(`WITH RECURSIVE hours(i) AS (VALUES(0) UNION ALL SELECT i+1 FROM hours WHERE i+1<?)
					INSERT INTO monitor_hourly(series_id,hour,payload,updated_at) SELECT ?,?+3600*i,'{"slots":[]}',? FROM hours`, tc.newerHours, series.ID, now.AddDate(0, 0, -50).Unix(), now.Unix())
				if err != nil {
					t.Fatal(err)
				}
			}
			if err := s.CleanupMonitor(ctx, now); !errors.Is(err, ErrMonitorCleanupPending) {
				t.Fatalf("first cleanup: %v", err)
			}
			stats := retentionStorage(t, s, "scope")
			if stats.RawSamples != 100 || stats.Hourly != tc.newerHours+4 || stats.PendingHours != 0 {
				t.Errorf("after first batch: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
			}
			if err := s.CleanupMonitor(ctx, now); err != nil {
				t.Fatal(err)
			}
			stats = retentionStorage(t, s, "scope")
			if stats.RawSamples != 0 || stats.Hourly != tc.newerHours || stats.PendingHours != 0 {
				t.Fatalf("after second batch: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
			}
		})
	}
}

func TestMonitorCleanupRepairsOrphanBaselinesInBatches(t *testing.T) {
	path := filepath.Join(t.TempDir(), "db")
	s, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { s.Close() })
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	start := now.AddDate(0, 0, -120)
	const hours = 300
	short := retentionSeries(t, s, "short", start)
	long := retentionSeries(t, s, "long", start)
	policy := defaultMonitorRetention()
	policy.AggregateDays = 365
	if _, err := s.SaveMonitorRetention(ctx, "long", policy); err != nil {
		t.Fatal(err)
	}
	seedRetentionSlots(t, s, short, 0, 30, hours)
	seedRetentionSlots(t, s, long, 0, 30, hours)
	// The old cleanup left raw baselines without hourly rows or dirty marks.
	if _, err := s.db.Exec(`DELETE FROM monitor_dirty_hours`); err != nil {
		t.Fatal(err)
	}
	err = s.CleanupMonitor(ctx, now)
	a, b := retentionStorage(t, s, "short"), retentionStorage(t, s, "long")
	queued := a.PendingHours + b.PendingHours
	if !errors.Is(err, ErrMonitorCleanupPending) || a.RawSamples != hours || b.RawSamples != hours || a.Hourly+b.Hourly != 0 || queued == 0 || queued > 24 {
		t.Fatalf("first repair: raw=%d/%d hourly=%d dirty=%d, err=%v", a.RawSamples, b.RawSamples, a.Hourly+b.Hourly, queued, err)
	}
	if err := s.Close(); err != nil {
		t.Fatal(err)
	}
	reopened, err := Open(path)
	if err != nil {
		t.Fatal(err)
	}
	s = reopened
	for i := 0; i < 40; i++ {
		if _, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil {
			t.Fatal(err)
		}
		err := s.CleanupMonitor(ctx, now)
		if err != nil && !errors.Is(err, ErrMonitorCleanupPending) {
			t.Fatal(err)
		}
		a, b = retentionStorage(t, s, "short"), retentionStorage(t, s, "long")
		if err == nil && a.RawSamples+b.RawSamples == 0 {
			break
		}
	}
	if a.RawSamples+b.RawSamples+a.Hourly+a.PendingHours+b.PendingHours != 0 || b.Hourly != hours {
		t.Fatalf("recovery did not converge: raw=%d/%d hourly=%d/%d dirty=%d/%d", a.RawSamples, b.RawSamples, a.Hourly, b.Hourly, a.PendingHours, b.PendingHours)
	}
	samples, err := s.SeriesSamples(ctx, long, start, now, false)
	if err != nil || len(samples) != hours {
		t.Fatalf("restored %d samples, %v", len(samples), err)
	}
	for i, sample := range samples {
		if sample.Slot != int64(i*30) || sample.DelayMS != i+1 || sample.Resolution != "hourly" {
			t.Fatalf("restored slot %d: %+v", i, sample)
		}
	}
}

func TestMonitorCleanupMaxHourlyWithRecentBaselines(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	start := now.Add(-150 * time.Hour)
	series := retentionPlanSeries(t, s, "scope", start, 8)
	policy := defaultMonitorRetention()
	policy.MaxHourly = 1000
	policy, err = s.SaveMonitorRetention(ctx, "scope", policy)
	if err != nil {
		t.Fatal(err)
	}
	for _, v := range series {
		seedRetentionSlots(t, s, v, 0, 30, 140)
	}
	// One early hour also has an archived-only slot. It is not redundant.
	seedRetentionSlots(t, s, series[0], 1, 1, 1)
	for i := 0; i < 1120; i += 24 {
		if _, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := s.db.Exec(`DELETE FROM monitor_observations WHERE series_id=? AND kind='baseline' AND slot=0`, series[0].ID); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 4; i++ {
		err := s.CleanupMonitor(ctx, now)
		if err != nil && !errors.Is(err, ErrMonitorCleanupPending) {
			t.Fatal(err)
		}
	}
	stats := retentionStorage(t, s, "scope")
	if stats.RawSamples != 1120 || stats.Hourly != 1000 || stats.PendingHours != 0 {
		t.Fatalf("recent evidence after cleanup: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
	}
	for i := 0; i < 2; i++ {
		if n, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil || n != 0 {
			t.Fatalf("rebuilding without new evidence: %d, %v", n, err)
		}
		if err := s.CleanupMonitor(ctx, now); err != nil {
			t.Fatal(err)
		}
	}
	// A late sample must still be merged exactly when its raw-only hour is sealed again.
	var slot int
	if err := s.db.QueryRow(`SELECT r.slot FROM monitor_observations r WHERE r.series_id=? AND r.kind='baseline'
		AND NOT EXISTS (SELECT 1 FROM monitor_hourly h WHERE h.series_id=r.series_id AND h.hour=r.scheduled_at/3600*3600)
		ORDER BY slot LIMIT 1`, series[1].ID).Scan(&slot); err != nil {
		t.Fatal(err)
	}
	seedRetentionSlots(t, s, series[1], slot+1, 1, 1)
	if n, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil || n != 1 {
		t.Fatalf("late rollup: %d, %v", n, err)
	}
	if err := s.CleanupMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	// Make room for archives. First force only the oldest 21 rows out under
	// MaxRawSamples, then expire the rest by time (including the late sample).
	policy.MaxHourly, policy.MaxRawSamples = 2000, 1100
	if _, err := s.SaveMonitorRetention(ctx, "scope", policy); err != nil {
		t.Fatal(err)
	}
	for _, step := range []struct {
		now time.Time
		raw int
	}{{now, 1100}, {now.AddDate(0, 0, 7), 0}} {
		for i := 0; i < 60; i++ {
			if _, err := s.AggregateMonitorBatch(ctx, step.now, 4); err != nil {
				t.Fatal(err)
			}
			err := s.CleanupMonitor(ctx, step.now)
			if err != nil && !errors.Is(err, ErrMonitorCleanupPending) {
				t.Fatal(err)
			}
			stats = retentionStorage(t, s, "scope")
			if err == nil && stats.RawSamples == step.raw {
				break
			}
		}
		if stats.RawSamples != step.raw || stats.PendingHours != 0 {
			t.Fatalf("raw quota %d: raw=%d dirty=%d", step.raw, stats.RawSamples, stats.PendingHours)
		}
	}
	for i, v := range series {
		samples, err := s.SeriesSamples(ctx, v, start, now, false)
		want := make(map[int64]int)
		for h := 0; h < 140; h++ {
			want[int64(h*30)] = h + 1
		}
		if i == 0 {
			want[1] = 1
		}
		if i == 1 {
			want[int64(slot+1)] = 1
		}
		if err != nil || len(samples) != len(want) {
			t.Fatalf("archived series %d: %d slots, want %d, %v", i, len(samples), len(want), err)
		}
		for _, sample := range samples {
			delay, ok := want[sample.Slot]
			if !ok || sample.Resolution != "hourly" || sample.Outcome != "success" || sample.DelayMS != delay || sample.ScheduledAt != v.Anchor+sample.Slot*120 {
				t.Fatalf("unexpected restored slot %+v", sample)
			}
			delete(want, sample.Slot)
		}
	}
}

func TestMonitorCleanupMaxHourlySkipsProtectedPrefix(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	series := retentionPlanSeries(t, s, "scope", now.Add(-504*time.Hour), 2)
	policy := defaultMonitorRetention()
	policy.RawDays, policy.MaxHourly = 30, 1000
	if _, err := s.SaveMonitorRetention(ctx, "scope", policy); err != nil {
		t.Fatal(err)
	}
	for _, v := range series {
		seedRetentionSlots(t, s, v, 0, 30, 501)
		seedRetentionSlots(t, s, v, 1, 30, 501)
	}
	for i := 0; i < 1002; i += 24 {
		if _, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil {
			t.Fatal(err)
		}
	}
	// Only the last hour can be reconstructed: all 1,001 preceding archives
	// hold at least one slot with no raw copy.
	if _, err := s.db.Exec(`DELETE FROM monitor_observations WHERE slot%30=0 AND NOT (series_id=? AND slot=15000)`, series[1].ID); err != nil {
		t.Fatal(err)
	}
	seedRetentionSlots(t, s, series[1], 15002, 1, 1)
	if err := s.CleanupMonitor(ctx, now); !errors.Is(err, ErrMonitorCleanupPending) {
		t.Fatalf("rollup can release capacity, got %v", err)
	}
	if n, err := s.AggregateMonitorBatch(ctx, now, 4); err != nil || n != 1 {
		t.Fatalf("late rollup: %d, %v", n, err)
	}
	if err := s.CleanupMonitor(ctx, now); err == nil || errors.Is(err, ErrMonitorCleanupPending) {
		t.Fatalf("expected capacity blocked on unique evidence, got %v", err)
	}
	stats := retentionStorage(t, s, "scope")
	if stats.Hourly != 1001 || stats.RawSamples != 1004 || stats.PendingHours != 0 {
		t.Fatalf("after all safe evictions: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
	}
}

func TestMonitorCleanupOverRawCapWaitsForRollup(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	now := time.Now().UTC().Truncate(time.Hour)
	series := retentionSeries(t, s, "scope", now.AddDate(0, 0, -180))
	policy := defaultMonitorRetention()
	policy.MaxRawSamples = 1000
	if _, err := s.SaveMonitorRetention(ctx, "scope", policy); err != nil {
		t.Fatal(err)
	}
	seedRetentionSlots(t, s, series, 0, 15, 1601) // Both raw limits apply even after a 500-row batch.
	err = s.CleanupMonitor(ctx, now)
	if err == nil || errors.Is(err, ErrMonitorCleanupPending) {
		t.Fatalf("expected blocked raw capacity, got %v", err)
	}
	stats := retentionStorage(t, s, "scope")
	if stats.RawSamples != 1601 || stats.Hourly != 0 || stats.PendingHours != 801 {
		t.Fatalf("unaggregated evidence: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
	}
	for i := 0; i < 801; i += 24 {
		if _, err := s.AggregateMonitorBatch(ctx, now, 24); err != nil {
			t.Fatal(err)
		}
	}
	if err := s.CleanupMonitor(ctx, now); !errors.Is(err, ErrMonitorCleanupPending) {
		t.Fatalf("expected bounded cleanup, got %v", err)
	}
	stats = retentionStorage(t, s, "scope")
	if stats.RawSamples != 1101 || stats.Hourly != 601 || stats.PendingHours != 0 {
		t.Fatalf("after bounded cleanup: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
	}
}

func TestMonitorCleanupPreservesDirtyAggregate(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	start := time.Now().UTC().Truncate(time.Hour).AddDate(0, 0, -100)
	series := retentionSeries(t, s, "scope", start)
	seedRetentionSlots(t, s, series, 0, 1, 1)
	if err := s.AggregateMonitor(ctx, start.AddDate(0, 0, 8)); err != nil {
		t.Fatal(err)
	}
	if err := s.CleanupMonitor(ctx, start.AddDate(0, 0, 8)); err != nil {
		t.Fatal(err)
	}
	seedRetentionSlots(t, s, series, 1, 1, 1)
	now := start.AddDate(0, 0, 100)
	if err := s.CleanupMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	stats := retentionStorage(t, s, "scope")
	if stats.RawSamples != 1 || stats.Hourly != 1 || stats.PendingHours != 1 {
		t.Fatalf("awaiting late sample: raw=%d hourly=%d dirty=%d", stats.RawSamples, stats.Hourly, stats.PendingHours)
	}
	if err := s.AggregateMonitor(ctx, now); err != nil {
		t.Fatal(err)
	}
	samples, err := s.SeriesSamples(ctx, series, start, now, false)
	if err != nil || len(samples) != 2 || samples[0].Slot != 0 || samples[0].Resolution != "hourly" || samples[1].Slot != 1 {
		t.Fatalf("late rollup: %+v, %v", samples, err)
	}
}
