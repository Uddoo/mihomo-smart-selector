package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"os"
	"path/filepath"
	"time"

	_ "modernc.org/sqlite"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type Store struct {
	db *sql.DB
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	store := &Store{db: db}
	if err := store.migrate(context.Background()); err != nil {
		db.Close()
		return nil, err
	}
	// Serialize short database operations: saving a binding can coincide with
	// scan completion, and competing SQLite writers otherwise return BUSY.
	db.SetMaxOpenConns(1)
	return store, nil
}

func (s *Store) Close() error {
	return s.db.Close()
}

func (s *Store) migrate(ctx context.Context) error {
	_, err := s.db.ExecContext(ctx, `
PRAGMA journal_mode = WAL;
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
	return s.ensureColumn(ctx, "scans", "profile_json", "TEXT NOT NULL DEFAULT '{}'")
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
	if _, err := s.db.ExecContext(ctx, `ALTER TABLE `+table+` ADD COLUMN `+column+` `+definition); err != nil {
		return fmt.Errorf("add %s.%s: %w", table, column, err)
	}
	return nil
}

func (s *Store) CreateScan(ctx context.Context, scan model.Scan) error {
	request, err := json.Marshal(scan.Request)
	if err != nil {
		return fmt.Errorf("encode scan request: %w", err)
	}
	profile, err := json.Marshal(scan.Profile)
	if err != nil {
		return fmt.Errorf("encode scan profile: %w", err)
	}
	_, err = s.db.ExecContext(ctx,
		`INSERT INTO scans(id, status, request_json, profile_json, started_at) VALUES (?, ?, ?, ?, ?)`,
		scan.ID, scan.Status, request, profile, timestamp(scan.StartedAt),
	)
	if err != nil {
		return fmt.Errorf("create scan: %w", err)
	}
	return nil
}

func (s *Store) CompleteScan(ctx context.Context, scan model.Scan) error {
	transaction, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return fmt.Errorf("begin complete scan: %w", err)
	}
	defer transaction.Rollback()

	var completedAt any
	if scan.CompletedAt != nil {
		completedAt = timestamp(*scan.CompletedAt)
	}
	if _, err := transaction.ExecContext(ctx,
		`UPDATE scans SET status = ?, completed_at = ?, error = ? WHERE id = ?`,
		scan.Status, completedAt, scan.Error, scan.ID,
	); err != nil {
		return fmt.Errorf("update scan: %w", err)
	}
	if _, err := transaction.ExecContext(ctx, `DELETE FROM scan_results WHERE scan_id = ?`, scan.ID); err != nil {
		return fmt.Errorf("clear old scan results: %w", err)
	}
	for _, result := range scan.Results {
		payload, err := json.Marshal(result)
		if err != nil {
			return fmt.Errorf("encode scan result: %w", err)
		}
		if _, err := transaction.ExecContext(ctx,
			`INSERT INTO scan_results(scan_id, rank, result_json) VALUES (?, ?, ?)`,
			scan.ID, result.Rank, payload,
		); err != nil {
			return fmt.Errorf("store scan result: %w", err)
		}
	}
	if err := transaction.Commit(); err != nil {
		return fmt.Errorf("commit complete scan: %w", err)
	}
	return nil
}

func (s *Store) GetScan(ctx context.Context, id string) (model.Scan, error) {
	var scan model.Scan
	var status string
	var requestJSON, profileJSON, startedAt, completedAt, errorText sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT status, request_json, profile_json, started_at, completed_at, error FROM scans WHERE id = ?`, id,
	).Scan(&status, &requestJSON, &profileJSON, &startedAt, &completedAt, &errorText)
	if err == sql.ErrNoRows {
		return model.Scan{}, fmt.Errorf("scan %q not found", id)
	}
	if err != nil {
		return model.Scan{}, fmt.Errorf("read scan: %w", err)
	}
	scan.ID = id
	scan.Status = model.ScanStatus(status)
	scan.Error = errorText.String
	if err := json.Unmarshal([]byte(requestJSON.String), &scan.Request); err != nil {
		return model.Scan{}, fmt.Errorf("decode scan request: %w", err)
	}
	if profileJSON.Valid && profileJSON.String != "" {
		if err := json.Unmarshal([]byte(profileJSON.String), &scan.Profile); err != nil {
			return model.Scan{}, fmt.Errorf("decode scan profile: %w", err)
		}
	}
	parsedStart, err := parseTimestamp(startedAt.String)
	if err != nil {
		return model.Scan{}, err
	}
	scan.StartedAt = parsedStart
	if completedAt.Valid {
		parsed, err := parseTimestamp(completedAt.String)
		if err != nil {
			return model.Scan{}, err
		}
		scan.CompletedAt = &parsed
	}

	rows, err := s.db.QueryContext(ctx, `SELECT result_json FROM scan_results WHERE scan_id = ? ORDER BY rank ASC`, id)
	if err != nil {
		return model.Scan{}, fmt.Errorf("read scan results: %w", err)
	}
	defer rows.Close()
	for rows.Next() {
		var payload string
		if err := rows.Scan(&payload); err != nil {
			return model.Scan{}, fmt.Errorf("read scan result: %w", err)
		}
		var result model.NodeResult
		if err := json.Unmarshal([]byte(payload), &result); err != nil {
			return model.Scan{}, fmt.Errorf("decode scan result: %w", err)
		}
		scan.Results = append(scan.Results, result)
	}
	if err := rows.Err(); err != nil {
		return model.Scan{}, fmt.Errorf("iterate scan results: %w", err)
	}
	return scan, nil
}

func (s *Store) RecordSwitch(ctx context.Context, event model.SwitchEvent) (model.SwitchEvent, error) {
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO switch_events(scan_id, group_name, previous_member, selected_member, reason, created_at) VALUES (?, ?, ?, ?, ?, ?)`,
		event.ScanID, event.Group, event.Previous, event.Selected, event.Reason, timestamp(event.CreatedAt),
	)
	if err != nil {
		return model.SwitchEvent{}, fmt.Errorf("store switch event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.SwitchEvent{}, fmt.Errorf("read switch event ID: %w", err)
	}
	event.ID = id
	return event, nil
}

func (s *Store) ListSwitches(ctx context.Context, limit int) ([]model.SwitchEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, scan_id, group_name, previous_member, selected_member, reason, created_at FROM switch_events ORDER BY id DESC LIMIT ?`,
		limit,
	)
	if err != nil {
		return nil, fmt.Errorf("list switch events: %w", err)
	}
	defer rows.Close()
	events := make([]model.SwitchEvent, 0)
	for rows.Next() {
		var event model.SwitchEvent
		var createdAt string
		if err := rows.Scan(&event.ID, &event.ScanID, &event.Group, &event.Previous, &event.Selected, &event.Reason, &createdAt); err != nil {
			return nil, fmt.Errorf("read switch event: %w", err)
		}
		parsed, err := parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		event.CreatedAt = parsed
		events = append(events, event)
	}
	return events, rows.Err()
}

func timestamp(value time.Time) string {
	return value.UTC().Format(time.RFC3339Nano)
}

func parseTimestamp(value string) (time.Time, error) {
	parsed, err := time.Parse(time.RFC3339Nano, value)
	if err != nil {
		return time.Time{}, fmt.Errorf("parse stored timestamp: %w", err)
	}
	return parsed, nil
}
