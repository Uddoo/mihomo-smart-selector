package scan

import (
	"context"
	"path/filepath"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// countingClient records how many Controller round-trips each catalog endpoint
// receives, which is the only thing a preflight is allowed to spend.
type countingClient struct {
	*fakeMihomo
	proxyReads    atomic.Int64
	providerReads atomic.Int64
	inFlight      atomic.Int64
	maxInFlight   atomic.Int64
}

func (c *countingClient) ListProxies(ctx context.Context) (map[string]mihomo.Proxy, error) {
	c.proxyReads.Add(1)
	c.enter()
	defer c.leave()
	// Widen the window so overlapping reads are observable, not just ordered.
	time.Sleep(20 * time.Millisecond)
	return c.fakeMihomo.ListProxies(ctx)
}

func (c *countingClient) ListProviders(ctx context.Context) ([]mihomo.Provider, error) {
	c.providerReads.Add(1)
	c.enter()
	defer c.leave()
	time.Sleep(20 * time.Millisecond)
	return c.fakeMihomo.ListProviders(ctx)
}

func (c *countingClient) enter() {
	current := c.inFlight.Add(1)
	for {
		peak := c.maxInFlight.Load()
		if current <= peak || c.maxInFlight.CompareAndSwap(peak, current) {
			return
		}
	}
}

func (c *countingClient) leave() { c.inFlight.Add(-1) }

func newCountingManager(t *testing.T) (*Manager, *countingClient) {
	t.Helper()
	cfg := config.Defaults()
	client := &countingClient{fakeMihomo: &fakeMihomo{
		proxies: map[string]mihomo.Proxy{
			"ChatGPT": {Name: "ChatGPT", Type: "Selector", All: []string{"JP-01", "JP-02"}, Now: "JP-01"},
			"JP-01":   {Name: "JP-01", Type: "Vless"},
			"JP-02":   {Name: "JP-02", Type: "Vless"},
		},
		delays: map[string]int{"JP-01": 90, "JP-02": 120},
	}}
	store, err := history.Open(filepath.Join(t.TempDir(), "preflight.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	return NewManager(cfg, client, store), client
}

func TestPreflightReusesCatalogAcrossFilterEdits(t *testing.T) {
	manager, client := newCountingManager(t)
	ctx := context.Background()
	request := model.ScanRequest{TargetGroup: "ChatGPT", Mode: "quick"}

	// A burst of filter edits is what the workbench actually produces: the same
	// target group with different region filters in quick succession.
	for _, regions := range [][]string{nil, {"JP"}, {"JP"}, nil, {"JP"}} {
		request.Regions = regions
		if preview, err := manager.Preflight(ctx, request); err != nil || !preview.Ready {
			t.Fatalf("preflight %v: %+v, %v", regions, preview, err)
		}
	}
	if got := client.proxyReads.Load(); got > 1 {
		t.Fatalf("preflight hit /proxies %d times for one burst; want at most 1", got)
	}
	if got := client.providerReads.Load(); got > 1 {
		t.Fatalf("preflight hit /providers %d times for one burst; want at most 1", got)
	}
}

func TestPreflightReadsCatalogsConcurrently(t *testing.T) {
	manager, client := newCountingManager(t)
	if _, err := manager.Preflight(context.Background(), model.ScanRequest{TargetGroup: "ChatGPT", Mode: "quick"}); err != nil {
		t.Fatal(err)
	}
	if peak := client.maxInFlight.Load(); peak < 2 {
		t.Fatalf("catalog reads peaked at %d in flight; want both endpoints overlapping", peak)
	}
}

// A scan is a measurement, so it must never be served from the preview cache.
func TestScanStillReadsFreshMembership(t *testing.T) {
	manager, client := newCountingManager(t)
	ctx := context.Background()
	if _, err := manager.Preflight(ctx, model.ScanRequest{TargetGroup: "ChatGPT", Mode: "quick"}); err != nil {
		t.Fatal(err)
	}
	before := client.proxyReads.Load()

	scan, err := manager.Start(model.ScanRequest{TargetGroup: "ChatGPT", Mode: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(5 * time.Second)
	for manager.hasRunningScan() && time.Now().Before(deadline) {
		time.Sleep(5 * time.Millisecond)
	}
	if got, err := manager.Get(ctx, scan.ID); err != nil || got.Status != model.ScanComplete {
		t.Fatalf("scan did not complete: %+v, %v", got, err)
	}
	if client.proxyReads.Load() <= before {
		t.Fatal("scan reused the preflight catalog cache instead of reading fresh membership")
	}
}

// Preflight must not queue behind the admission gate that Start holds.
func TestPreflightDoesNotWaitForAdmissionGate(t *testing.T) {
	manager, client := newCountingManager(t)
	manager.settingsMu.Lock()
	released := make(chan struct{})
	go func() {
		defer close(released)
		time.Sleep(150 * time.Millisecond)
		manager.settingsMu.Unlock()
	}()
	// The background goroutine owns the unlock; waiting here only keeps the
	// gate held long enough for the assertion to be meaningful.
	defer func() { <-released }()

	done := make(chan error, 1)
	go func() {
		_, err := manager.Preflight(context.Background(), model.ScanRequest{TargetGroup: "ChatGPT", Mode: "quick"})
		done <- err
	}()
	select {
	case err := <-done:
		if err != nil {
			t.Fatalf("preflight: %v", err)
		}
	case <-time.After(120 * time.Millisecond):
		t.Fatal("preflight blocked on the scan admission gate")
	}
	if client.proxyReads.Load() == 0 {
		t.Fatal("preflight never read the catalog")
	}
}
