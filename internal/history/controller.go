package history

import (
	"context"
	"strings"
)

// BindLegacyController associates pre-GUI single-Controller records with the
// original YAML destination, never with a newly saved GUI override. Existing
// scopes are immutable. Operators must retain their original YAML on upgrade.
func (s *Store) BindLegacyController(ctx context.Context, original string) error {
	scope := strings.TrimRight(original, "/")
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return err
	}
	defer tx.Rollback()
	if _, err := tx.ExecContext(ctx, `UPDATE scans SET controller_scope=? WHERE controller_scope=''`, scope); err != nil {
		return err
	}
	if _, err := tx.ExecContext(ctx, `UPDATE switch_events SET controller_scope=? WHERE controller_scope=''`, scope); err != nil {
		return err
	}
	return tx.Commit()
}
