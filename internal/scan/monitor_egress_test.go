package scan

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type egressRoundTrip func(*http.Request) (*http.Response, error)

func (f egressRoundTrip) RoundTrip(r *http.Request) (*http.Response, error) { return f(r) }

func TestMonitorEgressSelectorIdentityRestorationAndBudget(t *testing.T) {
	for _, mode := range []string{"normal", "during_probe_change", "protocol_changed", "post_read_error", "restore_noop", "budget", "unconfigured", "busy"} {
		t.Run(mode, func(t *testing.T) {
			m, f, _ := newVerificationFixture(t, "egress", mode)
			cfg := m.currentConfig()
			cfg.IPIntelligence = config.IPIntelligenceConfig{}
			if mode == "unconfigured" {
				cfg.EgressVerification.Enabled = false
			}
			m.cfg.Store(&cfg)
			if mode == "busy" {
				m.setActive(model.Scan{ID: "active", Status: model.ScanRunning})
			}
			m.monitorTraceTransport = func(proxy *url.URL) http.RoundTripper {
				if proxy.String() != cfg.EgressVerification.ProxyURL {
					t.Error("wrong proxy path")
				}
				return egressRoundTrip(func(r *http.Request) (*http.Response, error) {
					f.probeCalls.Add(1)
					if r.URL.String() != chatGPTTraceURL || f.current() != "candidate" {
						t.Error("trace did not use the chosen ChatGPT path")
					}
					if mode == "during_probe_change" {
						f.change(func(p *mihomo.Proxy) { p.Now = "external" })
					}
					if mode == "protocol_changed" {
						f.base.mu.Lock()
						p := f.base.proxies["candidate"]
						p.Type = "Trojan"
						f.base.proxies["candidate"] = p
						f.base.mu.Unlock()
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ip=2606:4700:4700::1111\nloc=US\n")), Header: make(http.Header), Request: r}, nil
				})
			}
			budget := 0
			v := m.MonitorEgress(context.Background(), "work", model.MonitorNode{Name: "candidate", Protocol: "VLESS"}, func() bool { budget++; return mode != "budget" })
			if mode == "normal" {
				if v.Status != "ok" || v.IP != "2606:4700:4700::1111" || v.Intelligence == nil || v.Intelligence.Status != "not_configured" {
					t.Fatal(v)
				}
			} else if v.Status == "ok" || v.IP != "" {
				t.Fatal("unconfirmed IP published", v)
			}
			_, writes := f.base.counts()
			if mode == "budget" || mode == "unconfigured" || mode == "busy" {
				if writes != 0 || f.probeCalls.Load() != 0 {
					t.Fatal("gated probe mutated selector")
				}
			}
			if mode == "normal" && (f.current() != "old" || writes != 2 || budget != 1) {
				t.Fatal("selector or request budget incorrect", f.current(), writes, budget)
			}
			if mode == "during_probe_change" && f.current() != "external" {
				t.Fatal("external selection overwritten")
			}
			f.base.mu.Lock()
			work := f.base.proxies["work"].Now
			f.base.mu.Unlock()
			if work != "candidate" {
				t.Fatal("business selector was changed")
			}
		})
	}
}

func TestChatGPTTraceParsesOnlyValidAddresses(t *testing.T) {
	for _, tc := range []struct {
		body  string
		valid bool
	}{
		{"ip=8.8.8.8\nloc=us\n", true}, {"ip=::ffff:8.8.8.8\nloc=US", true},
		{"ip=10.0.0.1\nloc=US", false}, {"ip=127.0.0.1", false}, {"ip=bad", false},
		{"ip=8.8.8.8\nip=1.1.1.1", false}, {"<html>blocked</html>", false},
	} {
		_, _, ok := parseChatGPTTrace(tc.body)
		if ok != tc.valid {
			t.Fatalf("%q: %v", tc.body, ok)
		}
	}
}
