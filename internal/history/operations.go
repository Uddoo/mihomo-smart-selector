package history

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (s *Store) switchWhere(ctx context.Context, clause string, args ...any) (model.SwitchEvent, error) {
	var event model.SwitchEvent
	var created string
	err := s.db.QueryRowContext(ctx, `SELECT id,scan_id,group_name,previous_member,selected_member,reason,created_at,status,request_id FROM switch_events WHERE `+clause, args...).Scan(&event.ID, &event.ScanID, &event.Group, &event.Previous, &event.Selected, &event.Reason, &created, &event.Status, &event.RequestID)
	if err != nil {
		return event, err
	}
	event.CreatedAt, err = parseTimestamp(created)
	event.AuditPersisted = true
	return event, err
}
func (s *Store) SwitchByRequest(ctx context.Context, key string) (model.SwitchEvent, error) {
	return s.switchWhere(ctx, "request_id=?", key)
}
func (s *Store) SwitchByID(ctx context.Context, id int64) (model.SwitchEvent, error) {
	return s.switchWhere(ctx, "id=?", id)
}

func (s *Store) LatestGroupSwitch(ctx context.Context, group string) (model.SwitchEvent, error) {
	return s.switchWhere(ctx, "group_name=? ORDER BY id DESC LIMIT 1", group)
}
func (s *Store) UnresolvedSwitch(ctx context.Context, group string) (bool, error) {
	var id int64
	err := s.db.QueryRowContext(ctx, `SELECT id FROM switch_events WHERE group_name=? AND status IN ('pending','unknown') LIMIT 1`, group).Scan(&id)
	if err == sql.ErrNoRows {
		return false, nil
	}
	return err == nil, err
}
func (s *Store) FinalizeSwitch(ctx context.Context, event model.SwitchEvent) error {
	result, err := s.db.ExecContext(ctx, `UPDATE switch_events SET status=?,reason=? WHERE id=?`, event.Status, event.Reason, event.ID)
	if err != nil {
		return err
	}
	n, err := result.RowsAffected()
	if err == nil && n != 1 {
		return fmt.Errorf("switch audit row unavailable")
	}
	return err
}
