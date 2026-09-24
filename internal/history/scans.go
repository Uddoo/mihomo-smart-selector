package history

import (
	"context"
	"database/sql"
	"encoding/json"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

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
		`INSERT INTO scans(id, status, request_json, profile_json, started_at, controller_scope) VALUES (?, ?, ?, ?, ?, ?)`,
		scan.ID, scan.Status, request, profile, timestamp(scan.StartedAt), scan.ControllerScope,
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

	progress, err := json.Marshal(scan.Progress)
	if err != nil {
		return err
	}
	warnings, err := json.Marshal(scan.Warnings)
	if err != nil {
		return err
	}
	var completedAt any
	if scan.CompletedAt != nil {
		completedAt = timestamp(*scan.CompletedAt)
	}
	if _, err := transaction.ExecContext(ctx,
		`UPDATE scans SET status = ?, completed_at = ?, error = ?, progress_json = ?, warnings_json = ? WHERE id = ?`,
		scan.Status, completedAt, scan.Error, progress, warnings, scan.ID,
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
	var requestJSON, profileJSON, startedAt, completedAt, errorText, progressJSON, warningsJSON sql.NullString
	err := s.db.QueryRowContext(ctx,
		`SELECT status, request_json, profile_json, started_at, completed_at, error, progress_json, warnings_json, controller_scope FROM scans WHERE id = ?`, id,
	).Scan(&status, &requestJSON, &profileJSON, &startedAt, &completedAt, &errorText, &progressJSON, &warningsJSON, &scan.ControllerScope)
	if err == sql.ErrNoRows {
		return model.Scan{}, fmt.Errorf("scan %q not found", id)
	}
	if err != nil {
		return model.Scan{}, fmt.Errorf("read scan: %w", err)
	}
	scan.ID = id
	scan.Status = model.ScanStatus(status)
	scan.Error = errorText.String
	if err := json.Unmarshal([]byte(warningsJSON.String), &scan.Warnings); err != nil {
		return model.Scan{}, fmt.Errorf("decode scan warnings: %w", err)
	}
	if err := json.Unmarshal([]byte(progressJSON.String), &scan.Progress); err != nil {
		return model.Scan{}, err
	}
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
