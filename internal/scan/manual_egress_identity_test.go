package scan

import (
	"context"
	"database/sql"
	"errors"
	"maps"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"sync"
	"sync/atomic"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// The delay and HTTP callbacks let the test replace identities at the two
// boundaries without timing assumptions. Snapshot values are never mutated.
type manualEgressController struct {
	mihomo.Client
	mu                        sync.Mutex
	proxies                   map[string]mihomo.Proxy
	providers                 []mihomo.Provider
	providerErr               error
	proxyReads, providerReads int
	writes                    []string
	onDelay                   func()
	onProviderRead            func()
}

func (c *manualEgressController) ListProxies(context.Context) (map[string]mihomo.Proxy, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.proxyReads++
	return maps.Clone(c.proxies), nil
}

func (c *manualEgressController) ListProviders(context.Context) ([]mihomo.Provider, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.providerReads++
	if c.onProviderRead != nil {
		c.onProviderRead()
	}
	return c.providers, c.providerErr
}

func (c *manualEgressController) Delay(context.Context, string, string, config.Probe, int) (int, error) {
	if c.onDelay != nil {
		c.onDelay()
	}
	return 15, nil
}

func (c *manualEgressController) Select(_ context.Context, group, name string) error {
	c.mu.Lock()
	defer c.mu.Unlock()
	c.writes = append(c.writes, group+"/"+name)
	p := c.proxies[group]
	p.Now = name
	c.proxies[group] = p
	return nil
}

func TestManualEgressIdentity(t *testing.T) {
	type change func(business, probe *manualEgressController)
	leaf := func(protocol, provider string) mihomo.Proxy {
		return mihomo.Proxy{Name: "node", Type: protocol, ProviderName: provider}
	}
	provider := func(owner, protocol string) mihomo.Provider {
		return mihomo.Provider{Name: owner, Proxies: []mihomo.Proxy{leaf(protocol, "")}}
	}
	duplicate := func(c *manualEgressController) {
		c.providers = []mihomo.Provider{provider("feed", "VLESS"), provider("other", "VLESS")}
	}
	setBothTrojan := func(b, p *manualEgressController) {
		b.proxies["node"], p.proxies["node"] = leaf("Trojan", ""), leaf("Trojan", "")
	}
	offline := errors.New("provider catalog unavailable")
	for _, tc := range []struct {
		name                              string
		initial, duringDelay, duringTrace change
		region, finalSelection            string
		traces                            int32
	}{
		{name: "matching", region: "JP", traces: 1},
		{name: "provider_only", initial: func(b, p *manualEgressController) {
			for _, c := range []*manualEgressController{b, p} {
				delete(c.proxies, "node")
				c.providers = []mihomo.Provider{provider("feed", "VLESS")}
			}
		}, region: "JP", traces: 1},
		{name: "proxy_provider_only", initial: func(b, p *manualEgressController) {
			b.proxies["node"], p.proxies["node"] = leaf("VLESS", "feed"), leaf("VLESS", "feed")
		}, region: "JP", traces: 1},
		{name: "wrong_protocol", initial: func(_, p *manualEgressController) { p.proxies["node"] = leaf("Trojan", "") }},
		{name: "wrong_provider", initial: func(b, p *manualEgressController) {
			b.providers, p.providers = []mihomo.Provider{provider("feed", "VLESS")}, []mihomo.Provider{provider("other", "VLESS")}
		}},
		{name: "wrong_proxy_provider", initial: func(b, p *manualEgressController) {
			b.proxies["node"], p.proxies["node"] = leaf("VLESS", "feed"), leaf("VLESS", "other")
		}},
		{name: "conflicting_catalogs", initial: func(b, p *manualEgressController) {
			b.providers, p.providers = []mihomo.Provider{provider("feed", "VLESS")}, []mihomo.Provider{provider("feed", "Trojan")}
		}},
		{name: "conflicting_owner_within_probe", initial: func(b, p *manualEgressController) {
			b.providers, p.providers = []mihomo.Provider{provider("feed", "VLESS")}, []mihomo.Provider{provider("feed", "VLESS")}
			p.proxies["node"] = leaf("VLESS", "other")
		}},
		{name: "missing_protocol", initial: func(b, p *manualEgressController) {
			b.proxies["node"], p.proxies["node"] = leaf("", ""), leaf("", "")
		}},
		{name: "map_key_disagrees_with_name", initial: func(_, p *manualEgressController) {
			p.proxies["node"] = mihomo.Proxy{Name: "other-node", Type: "VLESS"}
		}},
		{name: "external_selection_during_catalog_reads", finalSelection: "external", initial: func(_, p *manualEgressController) {
			p.onProviderRead = func() {
				p.proxies["probe"] = mihomo.Proxy{Name: "probe", Type: "Selector", Now: "external", All: []string{"old", "node", "external"}}
			}
		}},
		{name: "duplicate_provider", initial: func(_, p *manualEgressController) { duplicate(p) }},
		{name: "ambiguous_discovery_then_clean", initial: func(b, _ *manualEgressController) { duplicate(b) }, duringDelay: func(b, p *manualEgressController) {
			b.providers, p.providers = []mihomo.Provider{provider("other", "VLESS")}, []mihomo.Provider{provider("other", "VLESS")}
		}},
		{name: "discovery_read_error_then_clean", initial: func(b, _ *manualEgressController) { b.providerErr = offline }, duringDelay: func(b, _ *manualEgressController) { b.providerErr = nil }},
		{name: "missing_probe_leaf", initial: func(_, p *manualEgressController) { delete(p.proxies, "node") }},
		{name: "both_changed_after_discovery", duringDelay: setBothTrojan},
		{name: "removed_before_write", duringDelay: func(b, _ *manualEgressController) { delete(b.proxies, "work") }},
		{name: "provider_read_error_before_write", duringDelay: func(_, p *manualEgressController) { p.providerErr = offline }},
		{name: "business_changed_during_trace", duringTrace: func(b, _ *manualEgressController) { b.proxies["node"] = leaf("Trojan", "") }, traces: 1},
		{name: "probe_changed_during_trace", duringTrace: func(_, p *manualEgressController) { p.proxies["node"] = leaf("VLESS", "other") }, traces: 1},
		{name: "membership_changed_during_trace", duringTrace: func(b, _ *manualEgressController) {
			b.proxies["work"] = mihomo.Proxy{Name: "work", Type: "Selector", All: []string{"old"}, Now: "old"}
		}, traces: 1},
	} {
		t.Run(tc.name, func(t *testing.T) {
			business := &manualEgressController{proxies: map[string]mihomo.Proxy{"work": {Name: "work", Type: "Selector", All: []string{"old", "node"}, Now: "old"}, "node": leaf("VLESS", "")}}
			probe := &manualEgressController{proxies: map[string]mihomo.Proxy{"probe": {Name: "probe", Type: "Selector", All: []string{"old", "node"}, Now: "old"}, "node": leaf("VLESS", "")}}
			apply := func(fn change) {
				if fn != nil {
					business.mu.Lock()
					probe.mu.Lock()
					fn(business, probe)
					probe.mu.Unlock()
					business.mu.Unlock()
				}
			}
			apply(tc.initial)
			business.onDelay = func() { apply(tc.duringDelay) }
			var traces atomic.Int32
			trace := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				traces.Add(1)
				probe.mu.Lock()
				selected := probe.proxies["probe"].Now
				probe.mu.Unlock()
				if selected != "node" {
					t.Errorf("trace ran with selection %q", selected)
				}
				apply(tc.duringTrace)
				_, _ = w.Write([]byte("loc=JP\n"))
			}))
			defer trace.Close()
			cfg := config.Defaults()
			cfg.Scanner.ProbeProfiles = append(cfg.Scanner.ProbeProfiles, config.ProbeProfile{ID: "regional", Label: "Regional", RequireRegion: true, ExpectedRegions: []string{"JP"}, Probes: []config.Probe{{Name: "delay", URL: "https://example.invalid", ExpectedStatus: "200"}}})
			cfg.EgressVerification = config.EgressConfig{Enabled: true, SelectorGroup: "probe", ProxyURL: trace.URL, TraceURL: "http://trace.invalid/"}
			store, err := history.Open(filepath.Join(t.TempDir(), "scan.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			m := NewManager(cfg, business, store).WithEgressController(probe)
			s, err := m.Start(model.ScanRequest{TargetGroup: "work", ProfileID: "regional", Mode: "quick", Nodes: []string{"node"}})
			if err != nil {
				t.Fatal(err)
			}
			m.workers.Wait()
			ctx := context.Background()
			s, err = store.GetScan(ctx, s.ID)
			if err != nil || s.Status != model.ScanComplete || len(s.Results) != 1 || len(s.Warnings) != 0 {
				t.Fatalf("scan=%+v err=%v", s, err)
			}
			r := s.Results[0]
			wantSource := ""
			if tc.region != "" {
				wantSource = "isolated_identity_v1"
			}
			if r.EgressEvidenceSource != wantSource {
				t.Errorf("persisted region source %q, want %q", r.EgressEvidenceSource, wantSource)
			}
			if r.VerifiedRegion != tc.region || (r.EgressError == "") != (tc.region != "") || r.SuccessRate != 1 {
				t.Errorf("region=%q error=%q success=%v; want region %q", r.VerifiedRegion, r.EgressError, r.SuccessRate, tc.region)
			}
			if tc.region == "" {
				if _, err := m.SelectRequest(ctx, s.ID, r.Name, "attempt"); err == nil {
					t.Error("RequireRegion selected candidate")
				}
				if _, err := store.SwitchByRequest(ctx, "attempt"); !errors.Is(err, sql.ErrNoRows) {
					t.Errorf("unexpected switch audit: %v", err)
				}
			}
			var wantWrites []string
			if tc.traces > 0 {
				wantWrites = []string{"probe/node", "probe/old"}
			}
			wantSelection := tc.finalSelection
			if wantSelection == "" {
				wantSelection = "old"
			}
			if len(business.writes) != 0 || !reflect.DeepEqual(probe.writes, wantWrites) || traces.Load() != tc.traces || probe.proxies["probe"].Now != wantSelection {
				t.Errorf("business writes %v, probe writes %v, traces %d, final selection %q", business.writes, probe.writes, traces.Load(), probe.proxies["probe"].Now)
			}
			if tc.region != "" && (probe.proxyReads > 6 || probe.providerReads > 2 || business.proxyReads > 4 || business.providerReads > 3) {
				t.Errorf("redundant catalogs: primary %d/%d probe %d/%d", business.proxyReads, business.providerReads, probe.proxyReads, probe.providerReads)
			}
		})
	}
}
