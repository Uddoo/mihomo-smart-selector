package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"time"
)

func defaultMonitorRetention() model.MonitorRetention {
	return model.MonitorRetention{RawDays: 7, AggregateDays: 90, EventDays: 90, MaxRawSamples: 100000, MaxHourly: 20000}
}

func (s *Store) MonitorRetention(ctx context.Context, scope string) (model.MonitorRetention, error) {
	p := defaultMonitorRetention()
	var data string
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM monitor_retention WHERE scope=?`, scope).Scan(&data)
	if err == sql.ErrNoRows {
		return p, nil
	}
	if err != nil {
		return p, err
	}
	err = json.Unmarshal([]byte(data), &p)
	return p, err
}

func (s *Store) SaveMonitorRetention(ctx context.Context, scope string, p model.MonitorRetention) (model.MonitorRetention, error) {
	if p.RawDays < 1 || p.RawDays > 30 || p.AggregateDays < p.RawDays || p.AggregateDays < 7 || p.AggregateDays > 365 || p.EventDays < 7 || p.EventDays > 365 || p.MaxRawSamples < 1000 || p.MaxRawSamples > 1000000 || p.MaxHourly < 1000 || p.MaxHourly > 100000 {
		return p, fmt.Errorf("明细1–30天，聚合7–365天且不少于明细，事件7–365天；明细上限1000–1000000，聚合上限1000–100000")
	}
	previous := p.Revision
	p.Revision++
	data, _ := json.Marshal(p)
	var r sql.Result
	var err error
	if previous == 0 {
		r, err = s.db.ExecContext(ctx, `INSERT INTO monitor_retention(scope,revision,payload) VALUES(?,?,?) ON CONFLICT(scope) DO NOTHING`, scope, p.Revision, string(data))
	} else {
		r, err = s.db.ExecContext(ctx, `UPDATE monitor_retention SET revision=?,payload=? WHERE scope=? AND revision=?`, p.Revision, string(data), scope, previous)
	}
	if err != nil {
		return p, err
	}
	n, err := r.RowsAffected()
	if err == nil && n != 1 {
		err = fmt.Errorf("保留策略已变化，请重新读取")
	}
	return p, err
}

func (s *Store) MonitorStorage(ctx context.Context, scope string) (model.MonitorStorage, error) {
	out := model.MonitorStorage{}
	var err error
	out.Policy, err = s.MonitorRetention(ctx, scope)
	if err != nil {
		return out, err
	}
	for _, q := range []struct {
		sql  string
		dest *int
	}{
		{`SELECT COUNT(*) FROM monitor_observations r JOIN monitor_series t ON t.id=r.series_id WHERE t.scope=?`, &out.RawSamples},
		{`SELECT COUNT(*) FROM monitor_hourly h JOIN monitor_series t ON t.id=h.series_id WHERE t.scope=?`, &out.Hourly},
		{`SELECT COUNT(*) FROM monitor_events WHERE plan_id IN (SELECT plan_id FROM monitor_revisions WHERE scope=?)`, &out.Events},
		{`SELECT COUNT(*) FROM monitor_dirty_hours h JOIN monitor_series t ON t.id=h.series_id WHERE t.scope=?`, &out.PendingHours},
	} {
		if err = s.db.QueryRowContext(ctx, q.sql, scope).Scan(q.dest); err != nil {
			return out, err
		}
	}
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_samples r WHERE NOT EXISTS(SELECT 1 FROM monitor_bindings b WHERE b.plan_id=r.plan_id AND b.node_id=r.node_id)`).Scan(&out.UnmappedLegacy); err != nil {
		return out, err
	}
	for _, q := range []struct {
		sql  string
		dest **time.Time
	}{
		{`SELECT MIN(r.scheduled_at) FROM monitor_observations r JOIN monitor_series t ON t.id=r.series_id WHERE t.scope=?`, &out.OldestRaw},
		{`SELECT MIN(h.hour) FROM monitor_hourly h JOIN monitor_series t ON t.id=h.series_id WHERE t.scope=?`, &out.OldestHourly},
		{`SELECT MAX(h.updated_at) FROM monitor_hourly h JOIN monitor_series t ON t.id=h.series_id WHERE t.scope=?`, &out.LastAggregation},
	} {
		var value sql.NullInt64
		if err = s.db.QueryRowContext(ctx, q.sql, scope).Scan(&value); err != nil {
			return out, err
		}
		if value.Valid {
			v := time.Unix(value.Int64, 0).UTC()
			*q.dest = &v
		}
	}
	stats, err := s.Stats(ctx)
	out.DatabaseBytes = stats.DatabaseBytes
	out.WALBytes = stats.WALBytes
	return out, err
}

var ErrMonitorCleanupPending = errors.New("monitor cleanup has more bounded batches")

func retentionExec(ctx context.Context, tx *sql.Tx, query string, args ...any) (int, error) {
	r, err := tx.ExecContext(ctx, query, args...)
	if err != nil {
		return 0, err
	}
	n, err := r.RowsAffected()
	return int(n), err
}

// Only requeue missing archives when their raw rows are actually up for removal.
// This repairs legacy orphans without continually rebuilding recent hours whose
// redundant archives were intentionally evicted under MaxHourly.
func cleanupRawBatch(ctx context.Context, tx *sql.Tx, scope string, cutoff *int64, limit int, repairBudget *int) (deleted, queued int, err error) {
	observations := `SELECT rowid AS id,series_id,kind,scheduled_at,scheduled_at/3600*3600 AS hour
		FROM monitor_observations WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?)`
	args := []any{scope}
	if cutoff != nil {
		observations += ` AND scheduled_at<?`
		args = append(args, *cutoff)
	}
	candidates := `WITH due AS MATERIALIZED (` + observations + ` ORDER BY scheduled_at,rowid LIMIT ?) `
	args = append(args, limit)
	if *repairBudget > 0 {
		queued, err = retentionExec(ctx, tx, candidates+`INSERT INTO monitor_dirty_hours(series_id,hour)
			SELECT DISTINCT r.series_id,r.hour FROM due r WHERE r.kind='baseline'
			AND NOT EXISTS (SELECT 1 FROM monitor_hourly h WHERE h.series_id=r.series_id AND h.hour=r.hour)
			AND NOT EXISTS (SELECT 1 FROM monitor_dirty_hours d WHERE d.series_id=r.series_id AND d.hour=r.hour) LIMIT ?`, append(args, *repairBudget)...)
		if err != nil {
			return 0, 0, err
		}
		*repairBudget -= queued
	}
	const eligible = `r.kind!='baseline' OR (
		EXISTS (SELECT 1 FROM monitor_hourly h WHERE h.series_id=r.series_id AND h.hour=r.hour)
		AND NOT EXISTS (SELECT 1 FROM monitor_dirty_hours d WHERE d.series_id=r.series_id AND d.hour=r.hour))`
	if cutoff == nil {
		// Prefer the oldest raw rows, including those just sent for rollup.
		deleted, err = retentionExec(ctx, tx, candidates+`DELETE FROM monitor_observations WHERE rowid IN (
			SELECT r.id FROM due r WHERE `+eligible+`)`, args...)
		if err != nil || deleted > 0 || queued > 0 {
			return deleted, queued, err
		}
	}
	// All expired rows are due. Likewise a dirty oldest row should not cause
	// a hard raw-cap failure while another already sealed row can be reclaimed.
	deleted, err = retentionExec(ctx, tx, `DELETE FROM monitor_observations WHERE rowid IN (
		SELECT r.id FROM (`+observations+`) r WHERE `+eligible+` ORDER BY r.scheduled_at,r.id LIMIT ?)`, args...)
	return deleted, queued, err
}

func (s *Store) cleanupMonitorScope(ctx context.Context, scope string, p model.MonitorRetention, now time.Time, repairBudget *int) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	pending := false
	execute := func(query string, cap int, args ...any) error {
		n, e := retentionExec(ctx, tx, query, args...)
		if n >= cap {
			pending = true
		}
		return e
	}
	rawCutoff := now.Add(-time.Duration(p.RawDays) * 24 * time.Hour).Unix()
	deleted, queued, err := cleanupRawBatch(ctx, tx, scope, &rawCutoff, 500, repairBudget)
	if err != nil {
		return err
	}
	pending = deleted == 500 || queued > 0
	var rawCount int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_observations WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?)`, scope).Scan(&rawCount); err != nil {
		return err
	}
	if remaining := 500 - deleted; remaining > 0 && rawCount > p.MaxRawSamples {
		deleted, queued, err = cleanupRawBatch(ctx, tx, scope, nil, min(remaining, rawCount-p.MaxRawSamples), repairBudget)
		if err != nil {
			return err
		}
		rawCount -= deleted
		pending = pending || queued > 0 || deleted == remaining || (deleted > 0 && rawCount > p.MaxRawSamples)
	}
	// Expiry and overflow may discard an old archive after its raw batch drains.
	pruned, err := retentionExec(ctx, tx, `DELETE FROM monitor_hourly WHERE rowid IN (
		SELECT h.rowid FROM monitor_hourly h WHERE h.series_id IN (SELECT id FROM monitor_series WHERE scope=?)
		AND (h.hour<? OR h.rowid IN (SELECT older.rowid FROM monitor_hourly older JOIN monitor_series t ON t.id=older.series_id
			WHERE t.scope=? ORDER BY older.hour DESC,older.series_id LIMIT -1 OFFSET ?))
		AND NOT EXISTS (SELECT 1 FROM monitor_dirty_hours d WHERE d.series_id=h.series_id AND d.hour=h.hour)
		AND NOT EXISTS (SELECT 1 FROM monitor_observations r WHERE r.series_id=h.series_id AND r.kind='baseline'
			AND r.scheduled_at>=h.hour AND r.scheduled_at<h.hour+3600)
		ORDER BY h.hour LIMIT 200)`, scope, now.Add(-time.Duration(p.AggregateDays)*24*time.Hour).Unix(), scope, p.MaxHourly)
	if err != nil {
		return err
	}
	var hourlyCount int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_hourly WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?)`, scope).Scan(&hourlyCount); err != nil {
		return err
	}
	if pruned < 200 && hourlyCount > p.MaxHourly && rawCount <= p.MaxRawSamples {
		// A clean hour can still contain archived-only slots from an earlier raw
		// batch. Prove each compact slot has its immutable raw source before
		// evicting the copy. Never unseal a raw hour due for expiry this pass.
		n, err := retentionExec(ctx, tx, `DELETE FROM monitor_hourly WHERE rowid IN (
			SELECT h.rowid FROM monitor_hourly h INDEXED BY monitor_hourly_time
			WHERE h.series_id IN (SELECT id FROM monitor_series WHERE scope=?) AND h.hour>=?
			AND NOT EXISTS (SELECT 1 FROM monitor_observations r WHERE r.series_id=h.series_id AND r.kind='baseline'
				AND r.scheduled_at>=h.hour AND r.scheduled_at<h.hour+3600 AND r.scheduled_at<?)
			AND NOT EXISTS (SELECT 1 FROM monitor_dirty_hours d WHERE d.series_id=h.series_id AND d.hour=h.hour)
			AND EXISTS (SELECT 1 FROM monitor_observations r WHERE r.series_id=h.series_id AND r.kind='baseline'
				AND r.scheduled_at>=h.hour AND r.scheduled_at<h.hour+3600)
			AND NOT EXISTS (SELECT 1 FROM json_each(h.payload,'$.slots') slot WHERE NOT EXISTS (
				SELECT 1 FROM monitor_observations r WHERE r.series_id=h.series_id AND r.kind='baseline'
				AND r.slot=json_extract(slot.value,'$[0]') AND r.scheduled_at>=h.hour AND r.scheduled_at<h.hour+3600))
			ORDER BY h.hour,h.series_id LIMIT ?)`, scope, rawCutoff/3600*3600, rawCutoff, min(200-pruned, hourlyCount-p.MaxHourly))
		if err != nil {
			return err
		}
		pruned += n
		hourlyCount -= n
	}
	pending = pending || pruned == 200
	if !pending && hourlyCount > p.MaxHourly && rawCount <= p.MaxRawSamples {
		// Don't declare capacity blocked while the next aggregation batch can
		// still seal a late sample and make an existing archive reclaimable.
		err = tx.QueryRowContext(ctx, `SELECT EXISTS (SELECT 1 FROM monitor_dirty_hours d JOIN monitor_hourly h
			ON h.series_id=d.series_id AND h.hour=d.hour WHERE d.series_id IN (SELECT id FROM monitor_series WHERE scope=?)
			AND d.hour<=?)`, scope, rollupHourCutoff(now)).Scan(&pending)
		if err != nil {
			return err
		}
	}
	cutoff := now.Add(-time.Duration(p.EventDays) * 24 * time.Hour).Unix()
	if err = execute(`DELETE FROM monitor_events WHERE id IN (SELECT id FROM monitor_events WHERE plan_id IN (SELECT plan_id FROM monitor_revisions WHERE scope=?) AND at<? ORDER BY at LIMIT 500)`, 500, scope, cutoff); err != nil {
		return err
	}
	if err = execute(`DELETE FROM monitor_system_events WHERE id IN (SELECT id FROM monitor_system_events WHERE scope=? AND at<? ORDER BY at LIMIT 500)`, 500, scope, cutoff); err != nil {
		return err
	}
	if err = execute(`DELETE FROM monitor_correlations WHERE id IN (SELECT id FROM monitor_correlations WHERE scope=? AND updated_at<? AND status IN ('recovered','scope_changed') ORDER BY updated_at LIMIT 500)`, 500, scope, cutoff); err != nil {
		return err
	}
	if err = tx.Commit(); err != nil {
		return err
	}
	if pending {
		return ErrMonitorCleanupPending
	}
	if *repairBudget == 0 && (rawCount > p.MaxRawSamples || hourlyCount > p.MaxHourly) {
		return ErrMonitorCleanupPending
	}
	if hourlyCount > p.MaxHourly {
		return fmt.Errorf("小时聚合仍含唯一或待重算的证据，无法安全清理至容量上限")
	}
	if rawCount > p.MaxRawSamples {
		return fmt.Errorf("聚合尚未完成，保留唯一证据；请等待聚合后继续监控或提高容量上限")
	}
	return nil
}

func (s *Store) RecordMonitorSystem(ctx context.Context, scope, plan, status, message string, at time.Time) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO monitor_system_events(scope,plan_id,at,status,message) VALUES(?,?,?,?,?)`, scope, plan, at.Unix(), status, message)
	return err
}
