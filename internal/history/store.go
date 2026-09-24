package history

import (
	"context"
	"database/sql"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	_ "modernc.org/sqlite"
	"os"
	"path/filepath"
	"sync"
	"sync/atomic"
	"time"
)

type Store struct {
	db               *sql.DB
	path             string
	evidenceEpoch    atomic.Uint64
	evidenceMu       sync.Mutex
	evidenceVersions map[string]uint64
	taskCacheMu      sync.Mutex
	taskCache        map[string]map[string]model.MonitorTask
}

func Open(path string) (*Store, error) {
	if err := os.MkdirAll(filepath.Dir(path), 0o700); err != nil {
		return nil, fmt.Errorf("create data directory: %w", err)
	}
	db, err := sql.Open("sqlite", path)
	if err != nil {
		return nil, fmt.Errorf("open SQLite database: %w", err)
	}
	db.SetMaxOpenConns(1)
	store := &Store{db: db, path: path}
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
