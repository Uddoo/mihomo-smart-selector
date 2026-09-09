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

type blockingMihomo struct {
	*fakeMihomo
	started chan struct{}
	once    sync.Once
}

type cancelOnSelectMihomo struct {
	*fakeMihomo
	cancel context.CancelFunc
}

func (f *cancelOnSelectMihomo) Select(ctx context.Context, group, member string) error {
	if err := ctx.Err(); err != nil {
		return err
	}
	if member == "a" {
		f.cancel()
	}
	return f.fakeMihomo.Select(ctx, group, member)
}

func TestCancellationDuringVerificationDoesNotCompleteScan(t *testing.T) {
	for _, strict := range []bool{false, true} {
		t.Run(map[bool]string{false: "egress", true: "strict"}[strict], func(t *testing.T) {
			ctx, cancel := context.WithCancel(context.Background())
			defer cancel()
			cfg := config.Defaults()
			if strict {
				cfg.Scanner.StrictVerification = config.StrictVerificationConfig{Enabled: true, SelectorGroup: "probe", ProxyURL: "http://127.0.0.1:1", MaxCandidates: 10}
			} else {
				cfg.EgressVerification = config.EgressConfig{Enabled: true, SelectorGroup: "probe", ProxyURL: "http://127.0.0.1:1", TraceURL: "http://trace.test/"}
			}
			fake := &cancelOnSelectMihomo{fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{
				"ChatGPT": {Name: "ChatGPT", Type: "Selector", All: []string{"a"}},
				"a":       {Name: "a", Type: "VLESS"},
				"probe":   {Name: "probe", Type: "Selector", Now: "original", All: []string{"a", "original"}},
			}, delays: map[string]int{"a": 100}}, cancel: cancel}
			m := NewManager(cfg, fake, nil)
			_, err := m.scan(ctx, model.Scan{ID: "test", Request: model.ScanRequest{TargetGroup: "ChatGPT", ProfileID: "chatgpt", Mode: "quick"}})
			if !errors.Is(err, context.Canceled) {
				t.Fatalf("verification cancellation = %v", err)
			}
			if fake.proxies["probe"].Now != "original" {
				t.Fatal("probe selector not restored after cancellation")
			}
		})
	}
}

func (f *blockingMihomo) Delay(ctx context.Context, _, _ string, _ config.Probe, _ int) (int, error) {
	f.once.Do(func() { close(f.started) })
	<-ctx.Done()
	return 0, ctx.Err()
}

func TestCancelDuringBatchClosesResults(t *testing.T) {
	cfg := config.Defaults()
	cfg.Scanner.Concurrency = 1
	fake := &blockingMihomo{fakeMihomo: &fakeMihomo{}, started: make(chan struct{})}
	m := NewManager(cfg, fake, nil)
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() {
		m.probeBatch(ctx, model.Scan{ID: "cancel", Request: model.ScanRequest{Mode: "stable"}}, config.ProbeProfile{Probes: []config.Probe{{Name: "test"}}}, []candidate{{Name: "a"}, {Name: "b"}, {Name: "c"}})
		close(done)
	}()
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("cancelled batch did not drain and close")
	}
}

func TestStopPersistsCancelledScanAndReleasesSettingsLock(t *testing.T) {
	cfg := config.Defaults()
	cfg.Scanner.Concurrency = 1
	store, err := history.Open(t.TempDir() + "/cancel.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := &blockingMihomo{fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{
		"ChatGPT": {Name: "ChatGPT", Type: "Selector", All: []string{"a", "b"}},
		"a":       {Name: "a", Type: "VLESS"}, "b": {Name: "b", Type: "VLESS"},
	}}, started: make(chan struct{})}
	m := NewManager(cfg, fake, store)
	s, err := m.Start(model.ScanRequest{TargetGroup: "ChatGPT"})
	if err != nil {
		t.Fatal(err)
	}
	select {
	case <-fake.started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	if _, err := m.Stop(s.ID, false); err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for m.hasRunningScan() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.hasRunningScan() {
		t.Fatal("scan still running after cancellation")
	}
	s, err = store.GetScan(context.Background(), s.ID)
	if err != nil || s.Status != model.ScanCancelled {
		t.Fatalf("persisted scan = %+v, err = %v", s, err)
	}
	if _, err := m.SaveSettings(context.Background(), m.Settings()); err != nil {
		t.Fatalf("settings remained locked: %v", err)
	}
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

func TestCatalogClassifiesEntriesWithoutDeletingThem(t *testing.T) {
	cfg := config.Defaults()
	cfg.Regions = []config.Region{{Code: "JP", Name: "Japan", Aliases: []string{"JP"}}}
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{
		"香港 01":           {Name: "香港 01", Type: "Vless"},
		"LA香港中转01":        {Name: "LA香港中转01", Type: "Vless"},
		"DIRECT":          {Name: "DIRECT", Type: "Direct"},
		"过期时间：2030-01-01": {Name: "过期时间：2030-01-01", Type: "Shadowsocks"},
		"🌏自动最优线路":         {Name: "🌏自动最优线路", Type: "AnyTLS"},
		"香港组":             {Name: "香港组", Type: "Selector"},
	}}
	m := NewManager(cfg, fake, nil)
	nodes, err := m.Nodes(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	byName := map[string]model.NodeSummary{}
	for _, node := range nodes {
		byName[node.Name] = node
	}
	if len(nodes) != 6 {
		t.Fatalf("catalog must preserve 5 entries + provider node, but exclude group: %#v", nodes)
	}
	if byName["香港 01"].InferredRegion != "HK" || len(byName["香港 01"].RegionEvidence) == 0 {
		t.Fatalf("missing new region/evidence: %#v", byName["香港 01"])
	}
	if byName["LA香港中转01"].RegionSource != "ambiguous" || byName["LA香港中转01"].InferredRegion != "" {
		t.Fatal("transit must not assert egress country")
	}
	if byName["DIRECT"].EntryKind != "builtin" || byName["过期时间：2030-01-01"].EntryKind != "subscription-info" || byName["🌏自动最优线路"].RegionSource != "dynamic" {
		t.Fatal("entry kinds lost")
	}
	available := m.Regions()
	if len(available) != 40 {
		t.Fatalf("effective API dictionary: %d", len(available))
	}
	available[0].Aliases[0] = "mutated"
	if m.Regions()[0].Aliases[0] == "mutated" {
		t.Fatal("API leaked mutable classifier data")
	}
}

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
