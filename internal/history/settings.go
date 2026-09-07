package history

import (
	"context"
	"database/sql"
)

func (s *Store) RuntimeSettings(ctx context.Context, controller string) ([]byte, error) {
	var payload []byte
	err := s.db.QueryRowContext(ctx, `SELECT payload FROM runtime_settings WHERE controller = ?`, controller).Scan(&payload)
	if err == sql.ErrNoRows {
		return nil, nil
	}
	return payload, err
}

func (s *Store) SaveRuntimeSettings(ctx context.Context, controller string, payload []byte) error {
	_, err := s.db.ExecContext(ctx, `INSERT INTO runtime_settings(controller,payload) VALUES (?,?) ON CONFLICT(controller) DO UPDATE SET payload=excluded.payload`, controller, payload)
	return err
}
