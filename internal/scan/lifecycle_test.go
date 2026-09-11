package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type budgetController struct {
	*fakeMihomo
	active  atomic.Int32
	peak    atomic.Int32
	entered chan struct{}
}

func (f *budgetController) Delay(ctx context.Context, _, _ string, _ config.Probe, _ int) (int, error) {
	n := f.active.Add(1)
	defer f.active.Add(-1)
	for old := f.peak.Load(); n > old; old = f.peak.Load() {
		if f.peak.CompareAndSwap(old, n) {
			break
		}
	}
	select {
	case f.entered <- struct{}{}:
	default:
	}
	<-ctx.Done()
	return 0, ctx.Err()
}
func TestAdmissionGlobalBudgetAndShutdown(t *testing.T) {
	cfg := config.Defaults()
	cfg.Scanner.Concurrency = 2
	cfg.Scanner.MaxActiveScans = 2
	store, err := history.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := &budgetController{fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{}}, entered: make(chan struct{}, 4)}
	for i := 0; i < 4; i++ {
		name := fmt.Sprintf("n%d", i)
		fake.proxies[name] = mihomo.Proxy{Name: name, Type: "VLESS"}
	}
	for _, g := range []string{"g1", "g2", "g3"} {
		fake.proxies[g] = mihomo.Proxy{Name: g, Type: "Selector", All: []string{"n0", "n1", "n2", "n3"}}
	}
	m := NewManager(cfg, fake, store)
	first, err := m.Start(model.ScanRequest{TargetGroup: "g1"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(model.ScanRequest{TargetGroup: "g1"}); err == nil {
		t.Fatal("duplicate group admitted")
	}
	if _, err = m.Start(model.ScanRequest{TargetGroup: "g2"}); err != nil {
		t.Fatal(err)
	}
	if _, err = m.Start(model.ScanRequest{TargetGroup: "g3"}); err == nil {
		t.Fatal("active limit ignored")
	}
	for i := 0; i < 2; i++ {
		select {
		case <-fake.entered:
		case <-time.After(time.Second):
			t.Fatal("budget slots not used")
		}
	}
	if peak := fake.peak.Load(); peak != 2 {
		t.Fatalf("global concurrency=%d", peak)
	}
	if _, err = m.Cleanup(context.Background(), m.Settings().Revision); err == nil {
		t.Fatal("cleanup allowed during scan")
	}
	if err := m.PrepareRestart(); err == nil {
		t.Fatal("restart admitted during scan")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	if err = m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if fake.active.Load() != 0 {
		t.Fatal("probes left active")
	}
	if _, err = m.Start(model.ScanRequest{TargetGroup: "g3"}); err == nil {
		t.Fatal("start accepted during shutdown")
	}
	saved, err := store.GetScan(ctx, first.ID)
	if err != nil || saved.Status != model.ScanCancelled {
		t.Fatalf("shutdown state=%+v err=%v", saved, err)
	}
}

func TestPrepareRestartSealsNewWork(t *testing.T) {
	cfg := config.Defaults()
	store, err := history.Open(t.TempDir() + "/db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := NewManager(cfg, &fakeMihomo{}, store)
	if err := m.PrepareRestart(); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Start(model.ScanRequest{TargetGroup: "any"}); err == nil {
		t.Fatal("scan admitted after restart reservation")
	}
	if err := m.PrepareRestart(); err == nil {
		t.Fatal("duplicate restart reservation accepted")
	}
	if err := m.Shutdown(context.Background()); err != nil {
		t.Fatal(err)
	}
}

type uncertainController struct {
	*fakeMihomo
	calls     int
	readFails bool
	afterPut  func()
	putError  bool
}

func (f *uncertainController) Select(ctx context.Context, g, n string) error {
	f.calls++
	err := f.fakeMihomo.Select(ctx, g, n)
	if f.afterPut != nil {
		f.afterPut()
	}
	if f.putError {
		return errors.New("response lost")
	}
	return err
}
func (f *uncertainController) ListProxies(ctx context.Context) (map[string]mihomo.Proxy, error) {
	if f.readFails {
		return nil, errors.New("controller unavailable")
	}
	return f.fakeMihomo.ListProxies(ctx)
}

func TestSwitchWriteAheadReadbackIdempotencyAndUnknownReconciliation(t *testing.T) {
	cfg := config.Defaults()
	path := t.TempDir() + "/db"
	store, err := history.Open(path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	raw, err := sql.Open("sqlite", path)
	if err != nil {
		t.Fatal(err)
	}
	defer raw.Close()
	f := &uncertainController{fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{"g": {Name: "g", Type: "Selector", Now: "old", All: []string{"a"}}}}}
	m := NewManager(cfg, f, store)
	now := time.Now().UTC()
	s := model.Scan{ID: "s", ControllerScope: m.bindingScope(), Status: model.ScanComplete, StartedAt: now, CompletedAt: &now, Request: model.ScanRequest{TargetGroup: "g", ProfileID: "internet-baseline", Mode: "quick"}, Results: []model.NodeResult{{Name: "a", Rank: 1, SuccessRate: 1, MeasuredAt: now}}}
	if err = store.CreateScan(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err = store.CompleteScan(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if _, err = raw.Exec(`CREATE TRIGGER fail_insert BEFORE INSERT ON switch_events BEGIN SELECT RAISE(FAIL,'disk full fixture'); END`); err != nil {
		t.Fatal(err)
	}
	if _, err = m.SelectRequest(context.Background(), s.ID, "a", "insert-failure"); err == nil || f.calls != 0 {
		t.Fatal("switched without durable intent")
	}
	if _, err = raw.Exec(`DROP TRIGGER fail_insert`); err != nil {
		t.Fatal(err)
	}
	f.putError = true
	event, err := m.SelectRequest(context.Background(), s.ID, "a", "response-lost")
	if err != nil || event.Status != "confirmed" || !event.AuditPersisted {
		t.Fatalf("readback=%+v err=%v", event, err)
	}
	again, err := m.SelectRequest(context.Background(), s.ID, "a", "response-lost")
	if err != nil || again.ID != event.ID || f.calls != 1 {
		t.Fatal("idempotent retry repeated switch")
	}
	f.putError = false
	f.afterPut = func() { f.readFails = true }
	event, err = m.SelectRequest(context.Background(), s.ID, "a", "unknown")
	if err != nil || event.Status != "unknown" {
		t.Fatalf("unknown=%+v err=%v", event, err)
	}
	f.readFails = false
	f.afterPut = nil
	calls := f.calls
	if _, err = m.SelectRequest(context.Background(), s.ID, "a", "blocked"); err == nil || f.calls != calls {
		t.Fatal("unresolved switch did not block duplicate action")
	}
	event, err = m.ReconcileSwitch(context.Background(), event.ID)
	if err != nil || event.Status != "confirmed" || f.calls != calls {
		t.Fatal("reconciliation replayed or failed")
	}
	if _, err = raw.Exec(`CREATE TRIGGER fail_update BEFORE UPDATE ON switch_events BEGIN SELECT RAISE(FAIL,'disk full fixture'); END`); err != nil {
		t.Fatal(err)
	}
	event, err = m.SelectRequest(context.Background(), s.ID, "a", "finalize-failure")
	if err != nil || event.Status != "confirmed" || event.AuditPersisted {
		t.Fatalf("audit failure hidden: %+v %v", event, err)
	}
	saved, err := store.SwitchByRequest(context.Background(), "finalize-failure")
	if err != nil || saved.Status != "pending" {
		t.Fatal("pending evidence lost")
	}
}
