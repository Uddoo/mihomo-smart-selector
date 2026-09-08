package history

import (
	"context"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"os"
	"time"
)

type StorageStats struct {
	DatabaseBytes int64 `json:"database_bytes"`
	WALBytes      int64 `json:"wal_bytes"`
	Scans         int64 `json:"scans"`
	Audit         int64 `json:"audit"`
	Unresolved    int64 `json:"unresolved"`
}
type CleanupResult struct {
	Scans int64        `json:"scans_deleted"`
	Audit int64        `json:"audit_deleted"`
	Stats StorageStats `json:"stats"`
}

func (s *Store) Stats(ctx context.Context) (StorageStats, error) {
	var out StorageStats
	if err := s.db.QueryRowContext(ctx, `SELECT (SELECT COUNT(*) FROM scans),(SELECT COUNT(*) FROM switch_events),(SELECT COUNT(*) FROM switch_events WHERE status IN ('pending','unknown'))`).Scan(&out.Scans, &out.Audit, &out.Unresolved); err != nil {
		return out, err
	}
	if info, err := os.Stat(s.path); err == nil {
		out.DatabaseBytes = info.Size()
	}
	if info, err := os.Stat(s.path + "-wal"); err == nil {
		out.WALBytes = info.Size()
	}
	return out, nil
}

func (s *Store) Cleanup(ctx context.Context, p config.RetentionPolicy) (CleanupResult, error) {
	var out CleanupResult
	tx, err := s.db.BeginTx(ctx, nil)
	if err != nil {
		return out, err
	}
	defer tx.Rollback()
	result, err := tx.ExecContext(ctx, `DELETE FROM scans WHERE status NOT IN ('running','pending') AND id NOT IN (SELECT scan_id FROM switch_events WHERE status IN ('pending','unknown')) AND (julianday(started_at)<julianday(?) OR id IN (SELECT id FROM scans WHERE status NOT IN ('running','pending') ORDER BY started_at DESC LIMIT -1 OFFSET ?))`, timestamp(time.Now().AddDate(0, 0, -p.ScanDays)), p.MaxScans)
	if err != nil {
		return out, err
	}
	out.Scans, err = result.RowsAffected()
	if err != nil {
		return out, err
	}
	result, err = tx.ExecContext(ctx, `DELETE FROM switch_events WHERE status NOT IN ('pending','unknown') AND (julianday(created_at)<julianday(?) OR id IN (SELECT id FROM switch_events WHERE status NOT IN ('pending','unknown') ORDER BY id DESC LIMIT -1 OFFSET ?))`, timestamp(time.Now().AddDate(0, 0, -p.AuditDays)), p.MaxAudit)
	if err != nil {
		return out, err
	}
	out.Audit, err = result.RowsAffected()
	if err != nil {
		return out, err
	}
	if err = tx.Commit(); err != nil {
		return out, err
	}
	// Reclaim database pages only when cleanup removed data; no periodic rewrite
	// of a healthy database on flash storage.
	if out.Scans+out.Audit > 0 {
		if _, err = s.db.ExecContext(ctx, `VACUUM`); err != nil {
			return out, err
		}
	}
	if _, err = s.db.ExecContext(ctx, `PRAGMA wal_checkpoint(TRUNCATE)`); err != nil {
		return out, err
	}
	out.Stats, err = s.Stats(ctx)
	return out, err
}
