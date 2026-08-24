package scan

import (
	"context"
	"errors"
	"sync"
	"testing"
	"time"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
	"github.com/yw-li/mihomo-smart-selector/internal/history"
	"github.com/yw-li/mihomo-smart-selector/internal/mihomo"
	"github.com/yw-li/mihomo-smart-selector/internal/model"
)

type fakeMihomo struct {
	mu       sync.Mutex
	proxies  map[string]mihomo.Proxy
	selected string
	delays   map[string]int
}

func (f *fakeMihomo) ListProxies(context.Context) (map[string]mihomo.Proxy, error) {
	return f.proxies, nil
}
func (f *fakeMihomo) ListProviders(context.Context) ([]mihomo.Provider, error) {
	return []mihomo.Provider{{Name: "provider-a", Proxies: []mihomo.Proxy{{Name: "JP-03"}}}}, nil
}
func (f *fakeMihomo) Delay(_ context.Context, name, _ string, _ config.Probe, _ int) (int, error) {
	if delay, ok := f.delays[name]; ok {
		return delay, nil
	}
	return 0, errors.New("unreachable")
}
func (f *fakeMihomo) Select(_ context.Context, group, member string) error {
	f.mu.Lock()
	defer f.mu.Unlock()
	proxy := f.proxies[group]
	proxy.Now = member
	f.proxies[group] = proxy
	f.selected = member
	return nil
}
func (f *fakeMihomo) Reachable(context.Context) (string, error) { return "test", nil }

func TestManagerScansFiltersRanksAndSelectsMember(t *testing.T) {
	cfg := config.Defaults()
	cfg.Regions = []config.Region{{Code: "JP", Name: "Japan", Aliases: []string{"JP"}}}
	cfg.Storage.Path = t.TempDir() + "/selector.db"
	cfg.Scanner.Samples = 1
	cfg.Scanner.Probes = []config.Probe{{Name: "trace", URL: "https://chatgpt.com/cdn-cgi/trace", ExpectedStatus: "200"}}
	store, err := history.Open(cfg.Storage.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := &fakeMihomo{
		proxies: map[string]mihomo.Proxy{
			"ChatGPT":  {Name: "ChatGPT", Type: "Selector", Now: "JP-01", All: []string{"JP Smart", "JP-01", "JP-03"}},
			"JP Smart": {Name: "JP Smart", Type: "Smart"},
			"JP-01":    {Name: "JP-01", ProviderName: "provider-a"},
		},
		delays: map[string]int{"JP-01": 250, "JP-03": 110},
	}
	manager := NewManager(cfg, fake, store)
	scan, err := manager.Start(model.ScanRequest{TargetGroup: "ChatGPT", Regions: []string{"jp"}, Providers: []string{"provider-a"}, Mode: "stable"})
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for {
		scan, err = manager.Get(context.Background(), scan.ID)
		if err != nil {
			t.Fatal(err)
		}
		if scan.Status != model.ScanRunning {
			break
		}
		if time.Now().After(deadline) {
			t.Fatal("scan did not complete")
		}
		time.Sleep(10 * time.Millisecond)
	}
	if scan.Status != model.ScanComplete || len(scan.Results) != 2 || scan.Results[0].Name != "JP-03" {
		t.Fatalf("scan = %#v", scan)
	}
	event, err := manager.Select(context.Background(), scan.ID, "")
	if err != nil {
		t.Fatal(err)
	}
	if event.Selected != "JP-03" || fake.selected != "JP-03" {
		t.Fatalf("selection event = %#v, selected=%q", event, fake.selected)
	}
}
