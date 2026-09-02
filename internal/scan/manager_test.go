package scan

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"sync"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
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
	cfg.Scanner.BatchSize = 1
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

func TestStrictChecksDistinguishExpectedResponseFromRestrictedService(t *testing.T) {
	cfg := config.Defaults()
	cfg.Scanner.TimeoutMS = 1000
	store, err := history.Open(t.TempDir() + "/selector.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := NewManager(cfg, &fakeMihomo{}, store)

	passedServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusUnauthorized)
		_, _ = writer.Write([]byte("authorized boundary"))
	}))
	defer passedServer.Close()
	passed := manager.runStrictCheck(context.Background(), passedServer.Client(), config.StrictProbe{Name: "api", URL: passedServer.URL, ExpectedStatus: "401", BodyContains: "boundary"})
	if passed.Status != "passed" || passed.ObservedStatus != http.StatusUnauthorized || passed.BodyMatched == nil || !*passed.BodyMatched {
		t.Fatalf("passed strict check = %#v", passed)
	}

	restrictedServer := httptest.NewTLSServer(http.HandlerFunc(func(writer http.ResponseWriter, _ *http.Request) {
		writer.WriteHeader(http.StatusForbidden)
	}))
	defer restrictedServer.Close()
	restricted := manager.runStrictCheck(context.Background(), restrictedServer.Client(), config.StrictProbe{Name: "media", URL: restrictedServer.URL, ExpectedStatus: "200", RestrictedStatusCodes: []int{http.StatusForbidden}})
	if restricted.Status != "restricted" || restricted.ObservedStatus != http.StatusForbidden {
		t.Fatalf("restricted strict check = %#v", restricted)
	}

	result := model.NodeResult{StrictChecks: []model.StrictCheck{restricted}}
	applyStrictOutcome(&result)
	if result.StrictVerificationStatus != "restricted" || result.RestrictionStatus != "restricted" {
		t.Fatalf("strict outcome = %#v", result)
	}
}

func TestAssessResultKeepsServiceRegionAndTransportIndependent(t *testing.T) {
	result := model.NodeResult{SuccessRate: 1, VerifiedRegion: "US"}
	profile := config.ProbeProfile{
		ExpectedRegions: []string{"JP"}, TransportScope: "HTTP latency only",
		StrictProbes: []config.StrictProbe{{Name: "boundary", URL: "https://example.com", ExpectedStatus: "401"}},
	}
	assessResult(&result, profile, false)
	if result.ReachabilityStatus != "available" || result.RegionVerificationStatus != "mismatch" || result.StrictVerificationStatus != "not_configured" || result.TransportStatus != "HTTP latency only" {
		t.Fatalf("independent assessment = %#v", result)
	}
}

func TestPreflightKeepsProfileVisibleWhenSelectorHasNoDirectLeaves(t *testing.T) {
	cfg := config.Defaults()
	cfg.Storage.Path = t.TempDir() + "/selector.db"
	store, err := history.Open(cfg.Storage.Path)
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	manager := NewManager(cfg, &fakeMihomo{proxies: map[string]mihomo.Proxy{
		"GitHub":        {Name: "GitHub", Type: "Selector", All: []string{"nested-policy"}},
		"nested-policy": {Name: "nested-policy", Type: "URLTest"},
	}}, store)
	preview, err := manager.Preflight(context.Background(), model.ScanRequest{TargetGroup: "GitHub", Mode: "quick"})
	if err != nil {
		t.Fatal(err)
	}
	if preview.Ready || preview.Profile.ID != "github" || preview.Reason == "" {
		t.Fatalf("preflight = %#v", preview)
	}
}

func TestProfileSummaryShowsBuiltInTargetsButMasksPrivateTargets(t *testing.T) {
	cfg := config.Defaults()
	manager := NewManager(cfg, &fakeMihomo{}, nil)
	builtIn, err := cfg.ResolveProbeProfile("GLOBAL")
	if err != nil {
		t.Fatal(err)
	}
	public := manager.profileSummary(builtIn)
	if len(public.Targets) != 1 || !public.Targets[0].AddressVisible || public.Targets[0].Address != "https://www.gstatic.com/generate_204" || public.Targets[0].Kind != "reachability" {
		t.Fatalf("public target summary = %#v", public.Targets)
	}

	private := manager.profileSummary(config.ProbeProfile{
		ID: "emby", Label: "Private Emby", Probes: []config.Probe{{Name: "emby-info", URL: "https://media.example.test/emby/System/Info/Public", ExpectedStatus: "200"}},
	})
	if len(private.Targets) != 1 || private.Targets[0].AddressVisible || private.Targets[0].Address != "" {
		t.Fatalf("private target summary = %#v", private.Targets)
	}
}
