package history

import (
	"context"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (s *Store) Bindings(ctx context.Context, controller string) ([]model.ServiceBinding, error) {
	rows, err := s.db.QueryContext(ctx, `SELECT group_name, profile_id FROM service_bindings WHERE controller = ? ORDER BY group_name`, controller)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := []model.ServiceBinding{}
	for rows.Next() {
		var item model.ServiceBinding
		if err := rows.Scan(&item.Group, &item.ProfileID); err != nil {
			return nil, err
		}
		items = append(items, item)
	}
	return items, rows.Err()
}

func (s *Store) SetBinding(ctx context.Context, controller string, item model.ServiceBinding) error {
	if item.ProfileID == "" {
		_, err := s.db.ExecContext(ctx, `DELETE FROM service_bindings WHERE controller = ? AND group_name = ?`, controller, item.Group)
		return err
	}
	_, err := s.db.ExecContext(ctx, `INSERT INTO service_bindings(controller, group_name, profile_id) VALUES (?, ?, ?) ON CONFLICT(controller, group_name) DO UPDATE SET profile_id = excluded.profile_id`, controller, item.Group, item.ProfileID)
	return err
}
