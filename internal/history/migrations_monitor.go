package history

import (
	"context"
	"encoding/json"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (s *Store) migrateMonitorSeries(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
CREATE TABLE IF NOT EXISTS monitor_series (id TEXT PRIMARY KEY,scope TEXT NOT NULL,anchor INTEGER NOT NULL,payload TEXT NOT NULL);
CREATE INDEX IF NOT EXISTS monitor_series_scope ON monitor_series(scope);
CREATE TABLE IF NOT EXISTS monitor_bindings (plan_id TEXT NOT NULL,node_id TEXT NOT NULL,series_id TEXT NOT NULL,PRIMARY KEY(plan_id,node_id));
CREATE INDEX IF NOT EXISTS monitor_bindings_series ON monitor_bindings(series_id);
CREATE TABLE IF NOT EXISTS monitor_revisions (scope TEXT NOT NULL,revision INTEGER NOT NULL,plan_id TEXT NOT NULL,at INTEGER NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(scope,revision));
CREATE TABLE IF NOT EXISTS monitor_observations (series_id TEXT NOT NULL,kind TEXT NOT NULL,slot INTEGER NOT NULL,at INTEGER NOT NULL,scheduled_at INTEGER NOT NULL,payload TEXT NOT NULL,PRIMARY KEY(series_id,kind,slot));
CREATE INDEX IF NOT EXISTS monitor_observations_time ON monitor_observations(series_id,scheduled_at);
CREATE TABLE IF NOT EXISTS monitor_series_states (series_id TEXT PRIMARY KEY,payload TEXT NOT NULL);
CREATE TABLE IF NOT EXISTS monitor_hourly (series_id TEXT NOT NULL,hour INTEGER NOT NULL,version INTEGER NOT NULL DEFAULT 1,payload TEXT NOT NULL,updated_at INTEGER NOT NULL,PRIMARY KEY(series_id,hour));
CREATE TABLE IF NOT EXISTS monitor_dirty_hours (series_id TEXT NOT NULL,hour INTEGER NOT NULL,PRIMARY KEY(series_id,hour));
CREATE INDEX IF NOT EXISTS monitor_dirty_hours_time ON monitor_dirty_hours(hour,series_id);
CREATE TABLE IF NOT EXISTS monitor_migration (id INTEGER PRIMARY KEY,cursor INTEGER NOT NULL);
INSERT OR IGNORE INTO monitor_migration(id,cursor) VALUES(1,0);
`)
	if err != nil {
		return err
	}
	if err = s.migrateMonitorDiagnostics(ctx); err != nil {
		return err
	}
	if err = s.migrateMonitorTasks(ctx); err != nil {
		return err
	}
	rows, err := s.db.QueryContext(ctx, `SELECT scope,payload FROM monitor_plans`)
	if err != nil {
		return err
	}
	type legacy struct {
		scope string
		plan  model.MonitorPlan
	}
	plans := []legacy{}
	for rows.Next() {
		var scope, payload string
		if err = rows.Scan(&scope, &payload); err != nil {
			rows.Close()
			return err
		}
		var p model.MonitorPlan
		if err = json.Unmarshal([]byte(payload), &p); err != nil {
			rows.Close()
			return err
		}
		plans = append(plans, legacy{scope, p})
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return err
	}
	for _, item := range plans {
		if err = s.PrepareMonitorPlan(ctx, item.scope, &item.plan); err != nil {
			return err
		}
		tx, e := s.db.BeginTx(ctx, nil)
		if e != nil {
			return e
		}
		if e = s.persistMonitorSeries(ctx, tx, item.scope, item.plan); e == nil {
			var data []byte
			data, e = json.Marshal(item.plan)
			if e == nil {
				_, e = tx.ExecContext(ctx, `UPDATE monitor_plans SET payload=? WHERE scope=? AND task_id=?`, string(data), item.scope, item.plan.TaskID)
			}
		}
		if e != nil {
			tx.Rollback()
			return e
		}
		if e = tx.Commit(); e != nil {
			return e
		}
	}
	return s.importLegacyMonitor(ctx)
}

// Legacy rows without an immutable plan definition are deliberately not inferred.
// A durable rowid watermark makes each bounded import transaction restartable.
func (s *Store) importLegacyMonitor(ctx context.Context) error {
	for {
		rows, err := s.db.QueryContext(ctx, `SELECT r.rowid,b.series_id,t.anchor,r.payload FROM monitor_samples r JOIN monitor_bindings b ON b.plan_id=r.plan_id AND b.node_id=r.node_id JOIN monitor_series t ON t.id=b.series_id WHERE r.rowid>(SELECT cursor FROM monitor_migration WHERE id=1) ORDER BY r.rowid LIMIT 500`)
		if err != nil {
			return err
		}
		type item struct {
			seq    int64
			sample model.MonitorSample
		}
		batch := []item{}
		for rows.Next() {
			var seq, anchor int64
			var id, payload string
			if err = rows.Scan(&seq, &id, &anchor, &payload); err != nil {
				rows.Close()
				return err
			}
			var v model.MonitorSample
			if err = json.Unmarshal([]byte(payload), &v); err != nil {
				rows.Close()
				return err
			}
			v.SeriesID = id
			v.ScheduledAt = v.At.Unix()
			if v.Kind == "baseline" {
				v.ScheduledAt = anchor + v.Slot*120
			}
			v.DataVersion = 1
			v.ReasonCode = "legacy_unclassified"
			batch = append(batch, item{seq, v})
		}
		err = rows.Err()
		rows.Close()
		if err != nil {
			return err
		}
		if len(batch) == 0 {
			return nil
		}
		tx, err := s.db.BeginTx(ctx, nil)
		if err != nil {
			return err
		}
		for _, v := range batch {
			if _, err = insertObservation(ctx, tx, v.sample); err != nil {
				tx.Rollback()
				return err
			}
		}
		if _, err = tx.ExecContext(ctx, `UPDATE monitor_migration SET cursor=? WHERE id=1`, batch[len(batch)-1].seq); err != nil {
			tx.Rollback()
			return err
		}
		if err = tx.Commit(); err != nil {
			return err
		}
	}
}
