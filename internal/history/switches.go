package history

import (
	"context"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (s *Store) RecordSwitch(ctx context.Context, event model.SwitchEvent) (model.SwitchEvent, error) {
	if event.Status == "" {
		event.Status = "confirmed"
	}
	result, err := s.db.ExecContext(ctx,
		`INSERT INTO switch_events(scan_id, group_name, previous_member, selected_member, reason, created_at, status, request_id, controller_scope) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?)`,
		event.ScanID, event.Group, event.Previous, event.Selected, event.Reason, timestamp(event.CreatedAt), event.Status, event.RequestID, event.ControllerScope,
	)
	if err != nil {
		return model.SwitchEvent{}, fmt.Errorf("store switch event: %w", err)
	}
	id, err := result.LastInsertId()
	if err != nil {
		return model.SwitchEvent{}, fmt.Errorf("read switch event ID: %w", err)
	}
	event.ID = id
	event.AuditPersisted = true
	return event, nil
}

func (s *Store) ListSwitches(ctx context.Context, limit int) ([]model.SwitchEvent, error) {
	rows, err := s.db.QueryContext(ctx,
		`SELECT id, scan_id, group_name, previous_member, selected_member, reason, created_at, status, request_id, controller_scope FROM switch_events
 WHERE status IN ('pending','unknown') OR id IN (SELECT id FROM switch_events WHERE status NOT IN ('pending','unknown') ORDER BY id DESC LIMIT ?)
 ORDER BY CASE WHEN status IN ('pending','unknown') THEN 0 ELSE 1 END, id DESC`,
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
		if err := rows.Scan(&event.ID, &event.ScanID, &event.Group, &event.Previous, &event.Selected, &event.Reason, &createdAt, &event.Status, &event.RequestID, &event.ControllerScope); err != nil {
			return nil, fmt.Errorf("read switch event: %w", err)
		}
		parsed, err := parseTimestamp(createdAt)
		if err != nil {
			return nil, err
		}
		event.CreatedAt = parsed
		event.AuditPersisted = true
		events = append(events, event)
	}
	return events, rows.Err()
}
