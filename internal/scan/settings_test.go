package scan

import (
	"context"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestEgressVerificationRestoresProbeSelection(t *testing.T) {
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { _, _ = w.Write([]byte("loc=JP\n")) }))
	defer proxy.Close()
	cfg := config.Defaults()
	cfg.EgressVerification = config.EgressConfig{Enabled: true, SelectorGroup: "probe", ProxyURL: proxy.URL, TraceURL: "http://trace.test/"}
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{"probe": {Name: "probe", Type: "Selector", Now: "original", All: []string{"original", "candidate"}}}}
	manager := NewManager(cfg, fake, nil)
	results := []model.NodeResult{{Name: "candidate", InferredRegion: "JP"}}
	manager.verifyEgress(context.Background(), fake.proxies, results, "test")
	if results[0].VerifiedRegion != "JP" {
		t.Fatalf("exit not verified: %+v", results)
	}
	if fake.proxies["probe"].Now != "original" {
		t.Fatal("probe group selection not restored")
	}
}

func TestRuntimeSettingsPersistenceValidationAndScanLock(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	db := filepath.Join(t.TempDir(), "settings.db")
	store, err := history.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{"probe": {Name: "probe", Type: "Selector", All: []string{"node"}}, "work": {Name: "work", Type: "Selector", All: []string{"node"}}, "node": {Name: "node", Type: "VLESS"}}}
	manager := NewManager(cfg, fake, store)
	s := manager.Settings()
	s.Samples = 5
	s.BatchSize = 12
	s.DefaultProfile = "youtube"
	saved, err := manager.SaveSettings(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if saved.Revision != 1 || manager.currentConfig().Scanner.Samples != 5 {
		t.Fatal("settings not applied")
	}
	if _, err := manager.SaveSettings(ctx, s); err == nil {
		t.Fatal("stale browser overwrote newer settings")
	}
	invalid := saved
	invalid.Concurrency = 99
	if _, err := manager.SaveSettings(ctx, invalid); err == nil {
		t.Fatal("invalid concurrency accepted")
	}
	if manager.Settings().Revision != 1 {
		t.Fatal("invalid save changed configuration")
	}
	manager.setActive(model.Scan{ID: "running", Status: model.ScanRunning})
	if _, err := manager.SaveSettings(ctx, saved); err == nil {
		t.Fatal("settings changed during active scan")
	}
	manager.removeActive("running")
	if err := store.Close(); err != nil {
		t.Fatal(err)
	}
	store, err = history.Open(db)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager = NewManager(cfg, fake, store)
	if err := manager.LoadSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if manager.Settings().Samples != 5 || manager.Settings().Revision != 1 {
		t.Fatal("settings lost on restart")
	}
	other := cfg
	other.Mihomo.Controller = "http://other:9090"
	otherManager := NewManager(other, fake, store)
	if err := otherManager.LoadSettings(ctx); err != nil {
		t.Fatal(err)
	}
	if otherManager.Settings().Revision != 0 {
		t.Fatal("settings leaked to another controller")
	}

	s = manager.Settings()
	s.Egress = config.EgressConfig{Enabled: true, SelectorGroup: "missing", ProxyURL: "http://127.0.0.1:17890", TraceURL: "https://example.com/trace"}
	if _, err := manager.SaveSettings(ctx, s); err == nil {
		t.Fatal("missing probe group accepted")
	}
	s.Egress.SelectorGroup = "probe"
	saved, err = manager.SaveSettings(ctx, s)
	if err != nil {
		t.Fatal(err)
	}
	if fake.selected != "" {
		t.Fatal("saving settings mutated controller")
	}
	if _, err := manager.Preflight(ctx, model.ScanRequest{TargetGroup: "probe"}); err == nil {
		t.Fatal("probe selector accepted as business target")
	}
	if _, err := manager.Start(model.ScanRequest{TargetGroup: "probe"}); err == nil {
		t.Fatal("probe target scan started")
	}
	s = saved
	s.Egress.ProxyURL = "http://user:password@localhost:17890"
	if _, err := manager.SaveSettings(ctx, s); err == nil {
		t.Fatal("embedded proxy credentials accepted")
	}
	manager.setActive(model.Scan{ID: "running", Status: model.ScanRunning})
	if _, err := manager.Start(model.ScanRequest{TargetGroup: "work"}); err == nil {
		t.Fatal("parallel scans shared probe selector")
	}
	manager.removeActive("running")
}
