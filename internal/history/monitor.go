package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"errors"
	"fmt"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (s *Store) migrateMonitor(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS monitor_plans (scope TEXT PRIMARY KEY, revision INTEGER NOT NULL, payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS monitor_samples (
 plan_id TEXT NOT NULL, node_id TEXT NOT NULL, kind TEXT NOT NULL, slot INTEGER NOT NULL,
 at INTEGER NOT NULL, payload TEXT NOT NULL, PRIMARY KEY(plan_id,node_id,kind,slot));
CREATE INDEX IF NOT EXISTS monitor_samples_time ON monitor_samples(plan_id,at);
CREATE INDEX IF NOT EXISTS monitor_samples_retention ON monitor_samples(at);
CREATE TABLE IF NOT EXISTS monitor_states (plan_id TEXT NOT NULL,node_id TEXT NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(plan_id,node_id));
CREATE TABLE IF NOT EXISTS monitor_events (id INTEGER PRIMARY KEY AUTOINCREMENT,plan_id TEXT NOT NULL,at INTEGER NOT NULL,payload TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS monitor_events_time ON monitor_events(plan_id,at);
`)
	if err != nil {
		return err
	}
	return s.migrateMonitorSeries(ctx)
}

func (s *Store) MonitorPlan(ctx context.Context, scope string) (*model.MonitorPlan, error) {
	var payload string
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM monitor_plans WHERE scope=?`, scope).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p model.MonitorPlan
	err = json.Unmarshal([]byte(payload), &p)
	return &p, err
}

// Optimistic revision protects concurrent tabs and multiple processes.
func (s *Store) SaveMonitorPlan(ctx context.Context, scope string, p model.MonitorPlan, previous int) error {
	if err := s.PrepareMonitorPlan(ctx, scope, &p); err != nil {
		return err
	}
	payload, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var result sql.Result
	if previous == 0 {
		result, err = tx.ExecContext(ctx, `INSERT INTO monitor_plans(scope,revision,payload) VALUES(?,?,?) ON CONFLICT(scope) DO NOTHING`, scope, p.Revision, string(payload))
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE monitor_plans SET revision=?,payload=? WHERE scope=? AND revision=?`, p.Revision, string(payload), scope, previous)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return fmt.Errorf("监控配置已变化，请刷新后重试")
	}
	if err = s.persistMonitorSeries(ctx, tx, scope, p); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MonitorStates(ctx context.Context, plan string) (map[string]model.MonitorState, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT node_id,payload FROM monitor_states WHERE plan_id=? UNION ALL SELECT b.node_id,s.payload FROM monitor_bindings b JOIN monitor_series_states s ON s.series_id=b.series_id WHERE b.plan_id=?`, plan, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := map[string]model.MonitorState{}
	for rows.Next() {
		var id, payload string
		if err = rows.Scan(&id, &payload); err != nil {
			return nil, err
		}
		var v model.MonitorState
		if err = json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		out[id] = v
	}
	return out, rows.Err()
}

// Commit health and event only if the sample is new, in the same durable transaction.
func (s *Store) RecordMonitor(ctx context.Context, plan string, sample model.MonitorSample, state model.MonitorState, event *model.MonitorEvent) (bool, error) {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return false, err
	}
	defer tx.Rollback()
	var series string
	var anchor int64
	bindErr := tx.QueryRowContext(ctx, `SELECT b.series_id,t.anchor FROM monitor_bindings b JOIN monitor_series t ON t.id=b.series_id WHERE b.plan_id=? AND b.node_id=?`, plan, sample.NodeID).Scan(&series, &anchor)
	if bindErr == nil {
		sample.SeriesID = series
		sample.ScheduledAt = sample.At.Unix()
		if sample.Kind == "baseline" {
			sample.ScheduledAt = anchor + sample.Slot*120
		}
		sample.DataVersion = 2
		if sample.ReasonCode == "" {
			switch sample.Outcome {
			case "success":
				sample.ReasonCode = "ok"
			case "failure":
				sample.ReasonCode = "node_path"
			default:
				sample.ReasonCode = "unknown"
			}
		}
		added, e := insertObservation(ctx, tx, sample)
		if e != nil || !added {
			return added, e
		}
		data, e := json.Marshal(state)
		if e != nil {
			return false, e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO monitor_series_states(series_id,payload) VALUES(?,?) ON CONFLICT(series_id) DO UPDATE SET payload=excluded.payload`, series, string(data)); e != nil {
			return false, e
		}
		if event != nil {
			data, e = json.Marshal(event)
			if e != nil {
				return false, e
			}
			if _, e = tx.ExecContext(ctx, `INSERT INTO monitor_events(plan_id,at,payload) VALUES(?,?,?)`, plan, event.At.Unix(), string(data)); e != nil {
				return false, e
			}
		}
		return true, tx.Commit()
	}
	if bindErr != sql.ErrNoRows {
		return false, bindErr
	}
	data, err := json.Marshal(sample)
	if err != nil {
		return false, err
	}
	r, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_samples(plan_id,node_id,kind,slot,at,payload) VALUES(?,?,?,?,?,?)`, plan, sample.NodeID, sample.Kind, sample.Slot, sample.At.Unix(), string(data))
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return false, err
	}
	if n == 0 {
		return false, nil
	}
	data, err = json.Marshal(state)
	if err != nil {
		return false, err
	}
	if _, err = tx.ExecContext(ctx, `INSERT INTO monitor_states(plan_id,node_id,payload) VALUES(?,?,?) ON CONFLICT(plan_id,node_id) DO UPDATE SET payload=excluded.payload`, plan, sample.NodeID, string(data)); err != nil {
		return false, err
	}
	if event != nil {
		data, err = json.Marshal(event)
		if err != nil {
			return false, err
		}
		if _, err = tx.ExecContext(ctx, `INSERT INTO monitor_events(plan_id,at,payload) VALUES(?,?,?)`, plan, event.At.Unix(), string(data)); err != nil {
			return false, err
		}
	}
	return true, tx.Commit()
}

func (s *Store) MonitorSamples(ctx context.Context, plan string, since time.Time) ([]model.MonitorSample, error) {
	bindings, err := s.planSeries(ctx, plan)
	if err != nil {
		return nil, err
	}
	if len(bindings) > 0 {
		out := []model.MonitorSample{}
		for _, series := range bindings {
			items, e := s.SeriesSamples(ctx, series, since, time.Date(9999, 12, 31, 0, 0, 0, 0, time.UTC), true)
			if e != nil {
				return nil, e
			}
			out = append(out, items...)
		}
		sort.SliceStable(out, func(i, j int) bool { return out[i].At.Before(out[j].At) })
		return out, nil
	}
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM monitor_samples WHERE plan_id=? AND at>=? ORDER BY at,slot`, plan, since.Unix())
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorSample{}
	for rows.Next() {
		var payload string
		if err = rows.Scan(&payload); err != nil {
			return nil, err
		}
		var v model.MonitorSample
		if err = json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) MonitorEvents(ctx context.Context, plan string) ([]model.MonitorEvent, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,payload FROM monitor_events WHERE plan_id=? ORDER BY id DESC LIMIT 100`, plan)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorEvent{}
	for rows.Next() {
		var id int64
		var payload string
		if err = rows.Scan(&id, &payload); err != nil {
			return nil, err
		}
		var v model.MonitorEvent
		if err = json.Unmarshal([]byte(payload), &v); err != nil {
			return nil, err
		}
		v.ID = id
		out = append(out, v)
	}
	return out, rows.Err()
}

// Raw observations are removed only after their hour is durably aggregated.
// Never VACUUM in the sampling loop.
func (s *Store) CleanupMonitor(ctx context.Context, now time.Time) error {
	rows, err := s.db.QueryContext(ctx, `SELECT scope FROM monitor_plans UNION SELECT scope FROM monitor_series`)
	if err != nil {
		return err
	}
	scopes := []string{}
	for rows.Next() {
		var scope string
		if err = rows.Scan(&scope); err != nil {
			rows.Close()
			return err
		}
		scopes = append(scopes, scope)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	pending := false
	for _, scope := range scopes {
		p, e := s.MonitorRetention(ctx, scope)
		if e != nil {
			return e
		}
		if e = s.cleanupMonitorScope(ctx, scope, p, now); e != nil {
			if errors.Is(e, ErrMonitorCleanupPending) {
				pending = true
			} else {
				return e
			}
		}
	}
	r, err := s.db.ExecContext(ctx, `DELETE FROM monitor_samples WHERE rowid IN (SELECT rowid FROM monitor_samples WHERE at<? ORDER BY at LIMIT 500)`, now.Add(-7*24*time.Hour).Unix())
	if err != nil {
		return err
	}
	n, err := r.RowsAffected()
	if err != nil {
		return err
	}
	if pending || n >= 500 {
		return ErrMonitorCleanupPending
	}
	return nil
}
