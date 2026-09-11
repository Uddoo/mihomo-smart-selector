package history

import (
	"context"
	"encoding/json"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Run once before accepting requests. No network operation is replayed.
func (s *Store) RecoverInterrupted(ctx context.Context) error {
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err = tx.ExecContext(ctx, `UPDATE scans SET status='interrupted', completed_at=?, error='service restarted before scan completion' WHERE status IN ('running','pending')`, timestamp(time.Now())); err != nil {
		return err
	}
	if _, err = tx.ExecContext(ctx, `UPDATE switch_events SET status='unknown', reason='service restarted before switch outcome was persisted; reconcile before retrying' WHERE status='pending'`); err != nil {
		return err
	}
	return tx.Commit()
}

// Summaries omit result payloads; the selected scan is fetched separately.
func (s *Store) RecentScans(ctx context.Context, limit int) ([]model.Scan, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT id,status,request_json,profile_json,started_at,completed_at,error,progress_json,controller_scope FROM scans ORDER BY started_at DESC LIMIT ?`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []model.Scan{}
	for rows.Next() {
		var item model.Scan
		var request, profile, started, reason, progress string
		var completed *string
		if err := rows.Scan(&item.ID, &item.Status, &request, &profile, &started, &completed, &reason, &progress, &item.ControllerScope); err != nil {
			return nil, err
		}
		item.Error = reason
		if err := json.Unmarshal([]byte(request), &item.Request); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(profile), &item.Profile); err != nil {
			return nil, err
		}
		if err := json.Unmarshal([]byte(progress), &item.Progress); err != nil {
			return nil, err
		}
		item.StartedAt, err = parseTimestamp(started)
		if err != nil {
			return nil, err
		}
		if completed != nil {
			v, err := parseTimestamp(*completed)
			if err != nil {
				return nil, err
			}
			item.CompletedAt = &v
		}
		out = append(out, item)
	}
	return out, rows.Err()
}
