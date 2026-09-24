package history

import (
	"context"
	"encoding/json"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

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
