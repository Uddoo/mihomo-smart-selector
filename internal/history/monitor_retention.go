package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func defaultMonitorRetention() model.MonitorRetention {
	return model.MonitorRetention{RawDays: 7, AggregateDays: 90, EventDays: 90, MaxRawSamples: 100000, MaxHourly: 20000}
}

func (s *Store) migrateMonitorDiagnostics(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS monitor_retention (scope TEXT PRIMARY KEY,revision INTEGER NOT NULL,payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS monitor_system_events (id INTEGER PRIMARY KEY AUTOINCREMENT,scope TEXT NOT NULL,plan_id TEXT NOT NULL,at INTEGER NOT NULL,status TEXT NOT NULL,message TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS monitor_system_time ON monitor_system_events(scope,at);
CREATE TABLE IF NOT EXISTS monitor_correlation_state (scope TEXT NOT NULL,key TEXT NOT NULL,plan_id TEXT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(scope,key));
CREATE TABLE IF NOT EXISTS monitor_correlations (id INTEGER PRIMARY KEY AUTOINCREMENT,scope TEXT NOT NULL,plan_id TEXT NOT NULL,provider TEXT NOT NULL,status TEXT NOT NULL,started_at INTEGER NOT NULL,updated_at INTEGER NOT NULL,payload TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS monitor_correlation_time ON monitor_correlations(scope,updated_at);
`)
	return err
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

func (s *Store) cleanupMonitorScope(ctx context.Context, scope string, p model.MonitorRetention, now time.Time) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	pending := false
	execute := func(query string, cap int, args ...any) error {
		r, e := tx.ExecContext(ctx, query, args...)
		if e != nil {
			return e
		}
		n, e := r.RowsAffected()
		if n >= int64(cap) {
			pending = true
		}
		return e
	}
	protected := `(kind!='baseline' OR (EXISTS(SELECT 1 FROM monitor_hourly h WHERE h.series_id=monitor_observations.series_id AND h.hour=monitor_observations.scheduled_at/3600*3600) AND NOT EXISTS(SELECT 1 FROM monitor_dirty_hours d WHERE d.series_id=monitor_observations.series_id AND d.hour=monitor_observations.scheduled_at/3600*3600)))`
	if err = execute(`DELETE FROM monitor_observations WHERE rowid IN (SELECT rowid FROM monitor_observations WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?) AND scheduled_at<? AND `+protected+` ORDER BY scheduled_at LIMIT 500)`, 500, scope, now.Add(-time.Duration(p.RawDays)*24*time.Hour).Unix()); err != nil {
		return err
	}
	var rawCount int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_observations WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?)`, scope).Scan(&rawCount); err != nil {
		return err
	}
	if rawCount > p.MaxRawSamples {
		if err = execute(`DELETE FROM monitor_observations WHERE rowid IN (SELECT rowid FROM monitor_observations WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?) AND `+protected+` ORDER BY scheduled_at LIMIT ?)`, 500, scope, min(500, rawCount-p.MaxRawSamples)); err != nil {
			return err
		}
	}
	if err = execute(`DELETE FROM monitor_hourly WHERE rowid IN (SELECT rowid FROM monitor_hourly WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?) AND (hour<? OR rowid IN (SELECT h.rowid FROM monitor_hourly h JOIN monitor_series t ON t.id=h.series_id WHERE t.scope=? ORDER BY h.hour DESC,h.series_id LIMIT -1 OFFSET ?)) AND NOT EXISTS(SELECT 1 FROM monitor_dirty_hours d WHERE d.series_id=monitor_hourly.series_id AND d.hour=monitor_hourly.hour) ORDER BY hour LIMIT 200)`, 200, scope, now.Add(-time.Duration(p.AggregateDays)*24*time.Hour).Unix(), scope, p.MaxHourly); err != nil {
		return err
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
	if err = s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_observations WHERE series_id IN (SELECT id FROM monitor_series WHERE scope=?)`, scope).Scan(&rawCount); err != nil {
		return err
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
