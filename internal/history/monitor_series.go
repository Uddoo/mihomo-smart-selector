package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"time"
)

func (s *Store) PrepareMonitorPlan(ctx context.Context, scope string, p *model.MonitorPlan) error {
	// Plan values can share a caller-owned candidate slice. Own it before filling
	// identities/anchors so concurrent saves never mutate each other's input.
	p.Nodes = append([]model.MonitorNode(nil), p.Nodes...)
	if p.TaskID == "" {
		old, err := s.MonitorPlan(ctx, scope)
		if err != nil {
			return err
		}
		if old != nil {
			p.TaskID = old.TaskID
		} else {
			p.TaskID = monitorTaskID(scope, p.ID)
		}
	}
	namespace := p.TaskID
	err := s.db.QueryRowContext(ctx, `SELECT series_namespace FROM monitor_plans WHERE scope=? AND task_id=?`, scope, p.TaskID).Scan(&namespace)
	if err != nil && err != sql.ErrNoRows {
		return err
	}
	for i := range p.Nodes {
		n := &p.Nodes[i]
		parts := []string{scope, n.ID, n.Name, n.Provider, n.Protocol, p.ProfileID, p.ProfileHash, "https-baseline-120-v1"}
		if namespace != "" {
			parts = append(parts, namespace)
		}
		identity, _ := json.Marshal(parts)
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
	if _, err = tx.ExecContext(ctx, `INSERT INTO monitor_revisions(scope,task_id,revision,plan_id,at,payload) VALUES(?,?,?,?,?,?) ON CONFLICT(scope,task_id,revision) DO NOTHING`, scope, p.TaskID, p.Revision, p.ID, at.Unix(), string(data)); err != nil {
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

func (s *Store) MonitorSeries(ctx context.Context, scope string, taskIDs ...string) ([]model.MonitorSeries, error) {
	query := `SELECT payload FROM monitor_series WHERE scope=?`
	args := []any{scope}
	if len(taskIDs) > 0 {
		query += ` AND id IN (SELECT b.series_id FROM monitor_bindings b JOIN monitor_revisions r ON r.plan_id=b.plan_id WHERE r.scope=? AND r.task_id=?)`
		args = append(args, scope, taskIDs[0])
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY anchor,id`, args...)
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
	return s.monitorRevisions(ctx, scope, "")
}

func (s *Store) MonitorTaskRevisions(ctx context.Context, scope, taskID string) ([]model.MonitorRevision, error) {
	if _, err := s.MonitorTask(ctx, scope, taskID); err != nil {
		return nil, err
	}
	return s.monitorRevisions(ctx, scope, taskID)
}

func (s *Store) monitorRevisions(ctx context.Context, scope, taskID string) ([]model.MonitorRevision, error) {
	query := `SELECT at,payload FROM monitor_revisions WHERE scope=?`
	args := []any{scope}
	if taskID != "" {
		query += ` AND task_id=?`
		args = append(args, taskID)
	}
	rows, err := s.db.QueryContext(ctx, query+` ORDER BY id DESC LIMIT 200`, args...)
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

func (s *Store) SeriesDefinition(ctx context.Context, scope, id string, taskIDs ...string) (model.MonitorSeries, error) {
	var data string
	var out model.MonitorSeries
	query := `SELECT payload FROM monitor_series WHERE scope=? AND id=?`
	args := []any{scope, id}
	if len(taskIDs) > 0 {
		query += ` AND id IN (SELECT b.series_id FROM monitor_bindings b JOIN monitor_revisions r ON r.plan_id=b.plan_id WHERE r.scope=? AND r.task_id=?)`
		args = append(args, scope, taskIDs[0])
	}
	err := s.db.QueryRowContext(ctx, query, args...).Scan(&data)
	if err != nil {
		return out, fmt.Errorf("观测序列不存在")
	}
	err = json.Unmarshal([]byte(data), &out)
	return out, err
}
