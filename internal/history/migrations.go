package history

import (
	"context"
	"fmt"
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
CREATE INDEX IF NOT EXISTS monitor_events_plan_id ON monitor_events(plan_id,id);
`)
	if err != nil {
		return err
	}
	return s.migrateMonitorSeries(ctx)
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

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
PRAGMA journal_mode = WAL;
PRAGMA foreign_keys = ON;
CREATE TABLE IF NOT EXISTS scans (
  id TEXT PRIMARY KEY,
  status TEXT NOT NULL,
  request_json TEXT NOT NULL,
  started_at TEXT NOT NULL,
  completed_at TEXT,
  error TEXT NOT NULL DEFAULT ''
);
CREATE TABLE IF NOT EXISTS scan_results (
  scan_id TEXT NOT NULL,
  rank INTEGER NOT NULL,
  result_json TEXT NOT NULL,
  PRIMARY KEY (scan_id, rank),
  FOREIGN KEY (scan_id) REFERENCES scans(id) ON DELETE CASCADE
);
CREATE TABLE IF NOT EXISTS switch_events (
  id INTEGER PRIMARY KEY AUTOINCREMENT,
  scan_id TEXT NOT NULL,
  group_name TEXT NOT NULL,
  previous_member TEXT NOT NULL,
  selected_member TEXT NOT NULL,
  reason TEXT NOT NULL,
  created_at TEXT NOT NULL
);
CREATE INDEX IF NOT EXISTS idx_switch_events_created_at ON switch_events(created_at DESC);
CREATE TABLE IF NOT EXISTS service_bindings (
  controller TEXT NOT NULL,
  group_name TEXT NOT NULL,
  profile_id TEXT NOT NULL,
  PRIMARY KEY (controller, group_name)
);
CREATE TABLE IF NOT EXISTS runtime_settings (
 controller TEXT PRIMARY KEY,
 payload TEXT NOT NULL
);
`)
	if err != nil {
		return fmt.Errorf("migrate SQLite database: %w", err)
	}
	for _, column := range []struct{ table, name, definition string }{
		{"scans", "controller_scope", "TEXT NOT NULL DEFAULT ''"},
		{"switch_events", "controller_scope", "TEXT NOT NULL DEFAULT ''"},
		{"scans", "profile_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"scans", "progress_json", "TEXT NOT NULL DEFAULT '{}'"},
		{"scans", "warnings_json", "TEXT NOT NULL DEFAULT '[]'"},
		{"switch_events", "status", "TEXT NOT NULL DEFAULT 'confirmed'"},
		{"switch_events", "request_id", "TEXT NOT NULL DEFAULT ''"},
	} {
		if err := s.ensureColumn(ctx, column.table, column.name, column.definition); err != nil {
			return err
		}
	}
	_, err = s.db.ExecContext(ctx, `CREATE UNIQUE INDEX IF NOT EXISTS idx_switch_request ON switch_events(request_id) WHERE request_id <> ''; CREATE INDEX IF NOT EXISTS idx_scans_started ON scans(started_at DESC);`)
	if err != nil {
		return err
	}
	return s.migrateMonitor(ctx)
}

func (s *Store) ensureColumn(ctx context.Context, table, column, definition string) error {
	rows, err := s.db.QueryContext(ctx, `PRAGMA table_info(`+table+`)`)
	if err != nil {
		return fmt.Errorf("inspect %s columns: %w", table, err)
	}
	defer rows.Close()
	for rows.Next() {
		var index, notNull, primaryKey int
		var name, dataType string
		var defaultValue any
		if err := rows.Scan(&index, &name, &dataType, &notNull, &defaultValue, &primaryKey); err != nil {
			return fmt.Errorf("read %s columns: %w", table, err)
		}
		if name == column {
			return nil
		}
	}
	if err := rows.Err(); err != nil {
		return fmt.Errorf("iterate %s columns: %w", table, err)
	}
	if err := rows.Close(); err != nil {
		return err
	}
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}
