package scan

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestMonitorSharesGateAndSettingsCannotReplaceInFlightPool(t *testing.T) {
	cfg := config.Defaults()
	cfg.Scanner.Concurrency = 1
	store, err := history.Open(filepath.Join(t.TempDir(), "db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	f := &blockingMihomo{fakeMihomo: &fakeMihomo{}, started: make(chan struct{})}
	m := NewManager(cfg, f, store)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { m.MonitorDelay(ctx, model.MonitorNode{Name: "a"}, config.Probe{}); close(done) }()
	select {
	case <-f.started:
	case <-time.After(time.Second):
		t.Fatal("monitor did not acquire gate")
	}
	if _, err = m.MonitorDelay(ctx, model.MonitorNode{Name: "b"}, config.Probe{}); !errors.Is(err, ErrMonitorBusy) {
		t.Fatal(err)
	}
	s := m.Settings()
	s.Concurrency = 2
	if _, err = m.SaveSettings(ctx, s); err == nil {
		t.Fatal("changed pool under in-flight monitor")
	}
	s.Concurrency = 1
	s.Samples = 5
	if _, err = m.SaveSettings(ctx, s); err != nil {
		t.Fatal("unrelated setting cannot be saved", err)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("release used replacement gate")
	}
	s = m.Settings()
	s.Concurrency = 2
	if _, err = m.SaveSettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}
