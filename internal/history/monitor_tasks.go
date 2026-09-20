package history

import (
	"context"
	"crypto/sha256"
	"database/sql"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

var (
	ErrMonitorTaskRequired = errors.New("存在多个监控任务，请通过任务接口明确指定 task_id")
	ErrMonitorTaskNotFound = errors.New("监控任务不存在")
	ErrMonitorRevision     = errors.New("监控配置已变化，请刷新后重试")
	ErrMonitorGroupExists  = errors.New("此策略组已有监控任务")
)

func monitorTaskID(scope, seed string) string {
	data, _ := json.Marshal([]string{scope, seed})
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:16])
}

// Rebuild both singleton keys in one transaction. Evidence IDs, timestamps,
// revisions and series remain untouched. An empty namespace preserves legacy
// series hashes; newly created tasks use their stable task ID as namespace.
func (s *Store) migrateMonitorTasks(ctx context.Context) error {
	var migrated int
	if err := s.db.QueryRowContext(ctx, `SELECT COUNT(*) FROM pragma_table_info('monitor_plans') WHERE name='task_id'`).Scan(&migrated); err != nil || migrated != 0 {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	_, err = tx.ExecContext(ctx, `
ALTER TABLE monitor_plans RENAME TO monitor_plans_single;
ALTER TABLE monitor_revisions RENAME TO monitor_revisions_single;
CREATE TABLE monitor_plans (
 scope TEXT NOT NULL, task_id TEXT NOT NULL, group_name TEXT NOT NULL,
 revision INTEGER NOT NULL, payload TEXT NOT NULL, series_namespace TEXT NOT NULL,
 runtime INTEGER NOT NULL DEFAULT 0 CHECK(runtime IN (0,1)),
 PRIMARY KEY(scope,task_id), UNIQUE(scope,group_name));
CREATE UNIQUE INDEX monitor_runtime_scope ON monitor_plans(scope) WHERE runtime=1;
CREATE TABLE monitor_revisions (
 id INTEGER PRIMARY KEY AUTOINCREMENT, scope TEXT NOT NULL, task_id TEXT NOT NULL,
 revision INTEGER NOT NULL, plan_id TEXT NOT NULL, at INTEGER NOT NULL, payload TEXT NOT NULL,
 UNIQUE(scope,task_id,revision));
CREATE INDEX monitor_revisions_plan ON monitor_revisions(plan_id);
`)
	if err != nil {
		return err
	}
	type legacy struct {
		scope    string
		revision int
		at       int64
		plan     model.MonitorPlan
		payload  map[string]json.RawMessage
	}
	read := func(query string) ([]legacy, error) {
		rows, e := tx.QueryContext(ctx, query)
		if e != nil {
			return nil, e
		}
		defer rows.Close()
		var out []legacy
		for rows.Next() {
			var v legacy
			var data string
			if e = rows.Scan(&v.scope, &v.revision, &v.at, &data); e != nil {
				return nil, e
			}
			if e = json.Unmarshal([]byte(data), &v.plan); e != nil {
				return nil, e
			}
			v.plan.TaskID = monitorTaskID(v.scope, "legacy-monitor-task")
			if e = json.Unmarshal([]byte(data), &v.payload); e != nil {
				return nil, e
			}
			if v.payload == nil {
				return nil, fmt.Errorf("监控修订不是有效的方案对象")
			}
			v.payload["task_id"], e = json.Marshal(v.plan.TaskID)
			if e != nil {
				return nil, e
			}
			out = append(out, v)
		}
		return out, rows.Err()
	}
	plans, err := read(`SELECT scope,revision,0,payload FROM monitor_plans_single ORDER BY scope`)
	if err != nil {
		return err
	}
	revisions, err := read(`SELECT scope,revision,at,payload FROM monitor_revisions_single ORDER BY scope,revision`)
	if err != nil {
		return err
	}
	for _, v := range plans {
		data, e := json.Marshal(v.plan)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO monitor_plans(scope,task_id,group_name,revision,payload,series_namespace,runtime) VALUES(?,?,?,?,?,'',1)`, v.scope, v.plan.TaskID, v.plan.Group, v.revision, string(data)); e != nil {
			return e
		}
	}
	for _, v := range revisions {
		// Historical revisions are immutable evidence. Re-encoding today's
		// model would add defaults (for example candidate_limit) to older
		// revisions and discard fields this version does not understand.
		data, e := json.Marshal(v.payload)
		if e != nil {
			return e
		}
		if _, e = tx.ExecContext(ctx, `INSERT INTO monitor_revisions(scope,task_id,revision,plan_id,at,payload) VALUES(?,?,?,?,?,?)`, v.scope, v.plan.TaskID, v.revision, v.plan.ID, v.at, string(data)); e != nil {
			return e
		}
	}
	if _, err = tx.ExecContext(ctx, `DROP TABLE monitor_plans_single; DROP TABLE monitor_revisions_single;`); err != nil {
		return err
	}
	return tx.Commit()
}

func (s *Store) MonitorTasks(ctx context.Context, scope string) ([]model.MonitorTask, error) {
	// Read small revision headers on every request, including changes made by
	// another process. Decode a plan only when its revision actually changes.
	s.taskCacheMu.Lock()
	defer s.taskCacheMu.Unlock()
	rows, err := s.db.QueryContext(ctx, `SELECT task_id,revision,runtime FROM monitor_plans WHERE scope=? ORDER BY task_id`, scope)
	if err != nil {
		return nil, err
	}
	type header struct {
		id        string
		revision  int
		scheduled bool
	}
	headers := []header{}
	for rows.Next() {
		var h header
		if err = rows.Scan(&h.id, &h.revision, &h.scheduled); err != nil {
			rows.Close()
			return nil, err
		}
		headers = append(headers, h)
	}
	err = rows.Err()
	rows.Close()
	if err != nil {
		return nil, err
	}
	if s.taskCache == nil {
		s.taskCache = map[string]map[string]model.MonitorTask{}
	}
	previous := s.taskCache[scope]
	next := make(map[string]model.MonitorTask, len(headers))
	out := make([]model.MonitorTask, 0, len(headers))
	for _, h := range headers {
		task, ok := previous[h.id]
		if !ok || task.Plan.Revision != h.revision {
			task, err = s.MonitorTask(ctx, scope, h.id)
			if err != nil {
				return nil, err
			}
		} else {
			task.Scheduled = h.scheduled
		}
		next[h.id] = task
		task.Plan.Nodes = append([]model.MonitorNode{}, task.Plan.Nodes...)
		out = append(out, task)
	}
	s.taskCache[scope] = next
	return out, nil
}

func (s *Store) MonitorTask(ctx context.Context, scope, taskID string) (model.MonitorTask, error) {
	var task model.MonitorTask
	var data string
	err := s.db.QueryRowContext(ctx, `SELECT payload,runtime FROM monitor_plans WHERE scope=? AND task_id=?`, scope, taskID).Scan(&data, &task.Scheduled)
	if errors.Is(err, sql.ErrNoRows) {
		return task, ErrMonitorTaskNotFound
	}
	if err != nil {
		return task, err
	}
	err = json.Unmarshal([]byte(data), &task.Plan)
	return task, err
}

// Phase one keeps the existing runtime owner even when other paused tasks are
// configured. Restarting must neither choose an arbitrary task nor stop it.
func (s *Store) RuntimeMonitorPlan(ctx context.Context, scope string) (*model.MonitorPlan, error) {
	var data string
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM monitor_plans WHERE scope=? AND runtime=1`, scope).Scan(&data)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	var p model.MonitorPlan
	err = json.Unmarshal([]byte(data), &p)
	return &p, err
}

func (s *Store) SaveMonitorTask(ctx context.Context, scope string, p model.MonitorPlan, previous int) error {
	if p.TaskID == "" {
		return fmt.Errorf("task_id is required")
	}
	return s.saveMonitorTask(ctx, scope, p, previous, false)
}

func (s *Store) saveMonitorTask(ctx context.Context, scope string, p model.MonitorPlan, previous int, implicit bool) error {
	if previous < 0 || p.Revision != previous+1 || p.ID == "" {
		return ErrMonitorRevision
	}
	if err := s.PrepareMonitorPlan(ctx, scope, &p); err != nil {
		return err
	}
	data, err := json.Marshal(p)
	if err != nil {
		return err
	}
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	var count int
	if err = tx.QueryRowContext(ctx, `SELECT COUNT(*) FROM monitor_plans WHERE scope=?`, scope).Scan(&count); err != nil {
		return err
	}
	if implicit && count > 1 {
		return ErrMonitorTaskRequired
	}
	var owner string
	err = tx.QueryRowContext(ctx, `SELECT task_id FROM monitor_plans WHERE scope=? AND group_name=?`, scope, p.Group).Scan(&owner)
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return err
	}
	if err == nil && owner != p.TaskID {
		return ErrMonitorGroupExists
	}
	var result sql.Result
	if previous == 0 {
		if implicit && count != 0 {
			return ErrMonitorRevision
		}
		result, err = tx.ExecContext(ctx, `INSERT INTO monitor_plans(scope,task_id,group_name,revision,payload,series_namespace,runtime) VALUES(?,?,?,?,?,?,?) ON CONFLICT(scope,task_id) DO NOTHING`, scope, p.TaskID, p.Group, p.Revision, string(data), p.TaskID, count == 0)
	} else {
		result, err = tx.ExecContext(ctx, `UPDATE monitor_plans SET group_name=?,revision=?,payload=? WHERE scope=? AND task_id=? AND revision=?`, p.Group, p.Revision, string(data), scope, p.TaskID, previous)
	}
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n != 1 {
		return ErrMonitorRevision
	}
	if err = s.persistMonitorSeries(ctx, tx, scope, p); err != nil {
		return err
	}
	return tx.Commit()
}
