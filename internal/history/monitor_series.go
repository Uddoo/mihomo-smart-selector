package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"time"

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
				_, e = tx.ExecContext(ctx, `UPDATE monitor_plans SET payload=? WHERE scope=?`, string(data), item.scope)
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

func (s *Store) PrepareMonitorPlan(ctx context.Context, scope string, p *model.MonitorPlan) error {
	for i := range p.Nodes {
		n := &p.Nodes[i]
		identity, _ := json.Marshal([]string{scope, n.ID, n.Name, n.Provider, n.Protocol, p.ProfileID, p.ProfileHash, "https-baseline-120-v1"})
		sum := sha256.Sum256(identity)
		id := hex.EncodeToString(sum[:16])
		anchor := p.CreatedAt.Unix() + int64(i*120/len(p.Nodes))
		err := s.db.QueryRowContext(ctx, `SELECT anchor FROM monitor_series WHERE id=?`, id).Scan(&anchor)
		if err != nil && err != sql.ErrNoRows {
			return err
		}
		n.SeriesID = id
		n.Anchor = anchor
	}
	return nil
}

func (s *Store) persistMonitorSeries(ctx context.Context, tx *sql.Tx, scope string, p model.MonitorPlan) error {
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	at := p.UpdatedAt
	if at.IsZero() {
		at = p.CreatedAt
	}
	if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_revisions(scope,revision,plan_id,at,payload) VALUES(?,?,?,?,?)`, scope, p.Revision, p.ID, at.Unix(), string(data)); err != nil {
		return err
	}
	for _, n := range p.Nodes {
		series := model.MonitorSeries{ID: n.SeriesID, Node: n, ProfileID: p.ProfileID, ProfileHash: p.ProfileHash, Anchor: n.Anchor}
		data, err = json.Marshal(series)
		if err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_series(id,scope,anchor,payload) VALUES(?,?,?,?)`, n.SeriesID, scope, n.Anchor, string(data)); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_bindings(plan_id,node_id,series_id) VALUES(?,?,?)`, p.ID, n.ID, n.SeriesID); err != nil {
			return err
		}
		if _, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_series_states(series_id,payload) SELECT ?,payload FROM monitor_states WHERE plan_id=? AND node_id=?`, n.SeriesID, p.ID, n.ID); err != nil {
			return err
		}
	}
	return nil
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

func insertObservation(ctx context.Context, tx *sql.Tx, v model.MonitorSample) (bool, error) {
	data, err := json.Marshal(v)
	if err != nil {
		return false, err
	}
	r, err := tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_observations(series_id,kind,slot,at,scheduled_at,payload) VALUES(?,?,?,?,?,?)`, v.SeriesID, v.Kind, v.Slot, v.At.Unix(), v.ScheduledAt, string(data))
	if err != nil {
		return false, err
	}
	n, err := r.RowsAffected()
	if err != nil || n == 0 {
		return false, err
	}
	if v.Kind == "baseline" {
		_, err = tx.ExecContext(ctx, `INSERT OR IGNORE INTO monitor_dirty_hours(series_id,hour) VALUES(?,?)`, v.SeriesID, v.ScheduledAt/3600*3600)
	}
	return true, err
}

func (s *Store) MonitorSeries(ctx context.Context, scope string) ([]model.MonitorSeries, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT payload FROM monitor_series WHERE scope=? ORDER BY anchor,id`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorSeries{}
	for rows.Next() {
		var data string
		if err = rows.Scan(&data); err != nil {
			return nil, err
		}
		var v model.MonitorSeries
		if err = json.Unmarshal([]byte(data), &v); err != nil {
			return nil, err
		}
		out = append(out, v)
	}
	return out, rows.Err()
}

func (s *Store) MonitorRevisions(ctx context.Context, scope string) ([]model.MonitorRevision, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT at,payload FROM monitor_revisions WHERE scope=? ORDER BY revision DESC LIMIT 200`, scope)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.MonitorRevision{}
	for rows.Next() {
		var at int64
		var data string
		if err = rows.Scan(&at, &data); err != nil {
			return nil, err
		}
		var p model.MonitorPlan
		if err = json.Unmarshal([]byte(data), &p); err != nil {
			return nil, err
		}
		out = append(out, model.MonitorRevision{At: time.Unix(at, 0).UTC(), Plan: p})
	}
	return out, rows.Err()
}

func (s *Store) SeriesDefinition(ctx context.Context, scope, id string) (model.MonitorSeries, error) {
	var data string
	var out model.MonitorSeries
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM monitor_series WHERE scope=? AND id=?`, scope, id).Scan(&data)
	if err != nil {
		return out, fmt.Errorf("观测序列不存在")
	}
	err = json.Unmarshal([]byte(data), &out)
	return out, err
}
