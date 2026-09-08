package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type countingMihomo struct {
	*fakeMihomo
	calls atomic.Int32
}

func (f *countingMihomo) Delay(ctx context.Context, name, provider string, p config.Probe, timeout int) (int, error) {
	f.calls.Add(1)
	return f.fakeMihomo.Delay(ctx, name, provider, p, timeout)
}

func TestStagedScanBudgetsAndCurrentMember(t *testing.T) {
	cfg := config.Defaults()
	cfg.Scanner.RefineTopK = 2
	cfg.Scanner.Samples = 3
	fake := &countingMihomo{fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{}, delays: map[string]int{}}}
	names := []string{}
	for i := 0; i < 10; i++ {
		name := fmt.Sprintf("n%d", i)
		names = append(names, name)
		fake.proxies[name] = mihomo.Proxy{Name: name, Type: "VLESS"}
		fake.delays[name] = 100 + i*10
	}
	fake.proxies["work"] = mihomo.Proxy{Name: "work", Type: "Selector", Now: "n9", All: names}
	m := NewManager(cfg, fake, nil)
	s := model.Scan{ID: "test", Request: model.ScanRequest{TargetGroup: "work", ProfileID: "internet-baseline", Mode: "stable"}}
	m.setActive(s)
	results, err := m.scan(context.Background(), s)
	if err != nil {
		t.Fatal(err)
	}
	// 10 initial requests + 2 extra for each of top 2 and the current node.
	if got := fake.calls.Load(); got != 16 {
		t.Fatalf("requests = %d, want 16 instead of 30", got)
	}
	seen := map[string]bool{}
	for _, r := range results {
		if seen[r.Name] {
			t.Fatal("duplicate result")
		}
		seen[r.Name] = true
		want := 1
		if r.Name == "n0" || r.Name == "n1" || r.Name == "n9" {
			want = 3
			if r.Stage != "refined" {
				t.Fatal("missing refined stage")
			}
		}
		if len(r.Samples) != want {
			t.Fatalf("%s samples=%d", r.Name, len(r.Samples))
		}
		if r.MeasuredAt.IsZero() {
			t.Fatal("missing sample time")
		}
	}
	if results[2].Name != "n9" {
		t.Fatal("screened result ranked above refined current node")
	}
	live, _ := m.Get(context.Background(), s.ID)
	if len(live.Results) != 10 {
		t.Fatalf("live results=%d", len(live.Results))
	}
	if live.Progress.Completed != 13 || live.Progress.Total != 13 {
		t.Fatalf("progress=%+v", live.Progress)
	}
	fake.calls.Store(0)
	s.Request.Mode = "quick"
	quick, err := m.scan(context.Background(), s)
	if err != nil || fake.calls.Load() != 10 {
		t.Fatalf("quick mode changed: calls=%d, err=%v", fake.calls.Load(), err)
	}
	for _, r := range quick {
		if len(r.Samples) != 1 {
			t.Fatal("quick scan performed refinement")
		}
	}
}

func TestStrictBudgetVerifiesTopCandidatesInsteadOfSkippingAll(t *testing.T) {
	var requests atomic.Int32
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) { requests.Add(1); w.WriteHeader(200) }))
	defer proxy.Close()
	cfg := config.Defaults()
	cfg.Scanner.StrictVerification = config.StrictVerificationConfig{Enabled: true, SelectorGroup: "probe", ProxyURL: proxy.URL, MaxCandidates: 1}
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{"probe": {Name: "probe", Type: "Selector", Now: "original", All: []string{"a", "b", "original"}}}}
	m := NewManager(cfg, fake, nil)
	results := []model.NodeResult{{Name: "a", Score: 10}, {Name: "b", Score: 80}}
	m.verifyStrict(context.Background(), fake.proxies, config.ProbeProfile{StrictProbes: []config.StrictProbe{{Name: "check", URL: "http://test.invalid", ExpectedStatus: "200"}}}, results, "test")
	if requests.Load() != 1 || results[0].Name != "b" || results[0].StrictVerificationStatus != "passed" || results[1].StrictVerificationStatus != "not_run_limit" {
		t.Fatalf("budget outcome=%+v requests=%d", results, requests.Load())
	}
	if fake.proxies["probe"].Now != "original" {
		t.Fatal("probe not restored")
	}
}

func TestSelectionEnforcesFreshnessPolicyAndFreshMembership(t *testing.T) {
	cfg := config.Defaults()
	store, err := history.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{"work": {Name: "work", Type: "Selector", Now: "old", All: []string{"a"}}, "a": {Name: "a", Type: "VLESS"}}, delays: map[string]int{"a": 100}}
	m := NewManager(cfg, fake, store)
	now := time.Now().UTC()
	s := model.Scan{ID: "source", Status: model.ScanComplete, StartedAt: now, CompletedAt: &now, Request: model.ScanRequest{TargetGroup: "work", ProfileID: "internet-baseline", Mode: "stable"}, Results: []model.NodeResult{{Name: "a", Rank: 1, Stage: "refined", SuccessRate: 1, MeasuredAt: now.Add(-time.Hour)}}}
	if err := store.CreateScan(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteScan(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if _, err := m.Select(context.Background(), s.ID, "a"); err == nil {
		t.Fatal("selected expired result")
	}
	if fake.selected != "" {
		t.Fatal("expired selection changed controller")
	}
	retest, err := m.Retest(context.Background(), s.ID, "a")
	if err != nil {
		t.Fatal(err)
	}
	deadline := time.Now().Add(2 * time.Second)
	for m.hasRunningScan() && time.Now().Before(deadline) {
		time.Sleep(time.Millisecond)
	}
	if m.hasRunningScan() {
		t.Fatal("retest did not complete")
	}
	r, err := m.Get(context.Background(), retest.ID)
	if err != nil || r.Status != model.ScanComplete || len(r.Results) != 1 || r.Results[0].Stage != "refined" {
		t.Fatalf("retest=%+v err=%v", r, err)
	}
	if fake.selected != "" {
		t.Fatal("retest switched production selector")
	}
	if _, err := m.Groups(context.Background()); err != nil {
		t.Fatal(err)
	} // Seed catalog before membership changes.
	g := fake.proxies["work"]
	g.All = nil
	fake.proxies["work"] = g
	if _, err := m.Select(context.Background(), r.ID, "a"); err == nil {
		t.Fatal("selection used stale cached membership")
	}
	profile, _ := cfg.ProbeProfileByID("internet-baseline")
	profile.RequireStrict = true
	if m.selectionReason(r, r.Results[0], profile, now) == "" {
		t.Fatal("strict policy ignored")
	}
	profile.RequireStrict = false
	profile.RequireRegion = true
	profile.ExpectedRegions = []string{"JP"}
	node := r.Results[0]
	node.VerifiedRegion = "US"
	if m.selectionReason(r, node, profile, now) == "" {
		t.Fatal("region mismatch accepted")
	}
	node.VerifiedRegion = "JP"
	if reason := m.selectionReason(r, node, profile, now); reason != "" {
		t.Fatal(reason)
	}
	node.Stage = "screened"
	if m.selectionReason(r, node, profile, now) == "" {
		t.Fatal("screened stable candidate accepted")
	}
}

func TestDiscoveryCoalescesAndCopiesSnapshots(t *testing.T) {
	var cache discoveryCache[map[string]mihomo.Proxy]
	var calls atomic.Int32
	fetch := func(context.Context) (map[string]mihomo.Proxy, error) {
		calls.Add(1)
		return map[string]mihomo.Proxy{"group": {Name: "group", All: []string{"a"}}}, nil
	}
	var wg sync.WaitGroup
	for i := 0; i < 20; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			result, err := cache.get(context.Background(), fetch, cloneProxies)
			if err != nil {
				t.Error(err)
				return
			}
			result["group"].All[0] = "changed"
		}()
	}
	wg.Wait()
	if calls.Load() != 1 {
		t.Fatalf("discovery requests=%d", calls.Load())
	}
	result, _ := cache.get(context.Background(), fetch, cloneProxies)
	if result["group"].All[0] != "a" {
		t.Fatal("cache mutated by consumer")
	}
	cache.mu.Lock()
	cache.until = time.Time{}
	cache.mu.Unlock()
	_, _ = cache.get(context.Background(), fetch, cloneProxies)
	if calls.Load() != 2 {
		t.Fatal("expired cache not refreshed")
	}
}

func TestOldSavedSettingsInheritNewDefaults(t *testing.T) {
	cfg := config.Defaults()
	store, err := history.Open(t.TempDir() + "/test.db")
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	m := NewManager(cfg, &fakeMihomo{}, store)
	data, _ := json.Marshal(cfg.RuntimeSettings())
	var old map[string]any
	_ = json.Unmarshal(data, &old)
	delete(old, "refine_top_k")
	delete(old, "result_max_age_seconds")
	data, _ = json.Marshal(old)
	if err := store.SaveRuntimeSettings(context.Background(), m.bindingScope(), data); err != nil {
		t.Fatal(err)
	}
	if err := m.LoadSettings(context.Background()); err != nil {
		t.Fatal(err)
	}
	if m.Settings().RefineTopK != 10 || m.Settings().ResultMaxAgeSeconds != 600 {
		t.Fatal("new defaults lost during settings migration")
	}
}
