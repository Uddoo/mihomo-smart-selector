package scan

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestIsolatedEgressNeverWritesBusinessController(t *testing.T) {
	for _, mode := range []string{"ok", "offline", "wrong_protocol", "main_identity_changed"} {
		t.Run(mode, func(t *testing.T) {
			m, main, _ := newVerificationFixture(t, "egress", "normal")
			main.base.mu.Lock()
			delete(main.base.proxies, "probe")
			main.base.mu.Unlock()
			probe := &probeTargetController{proxies: map[string]mihomo.Proxy{
				"probe":     {Name: "probe", Type: "Selector", Now: "old", All: []string{"old", "candidate"}},
				"candidate": {Name: "candidate", Type: "VLESS"},
			}}
			server := httptest.NewServer(probe)
			defer server.Close()
			cfg := config.Defaults().Mihomo
			cfg.Controller = server.URL
			client, err := mihomo.NewWithSecret(cfg, "")
			if err != nil {
				t.Fatal(err)
			}
			m.WithEgressController(client)
			if mode == "offline" {
				server.Close()
			}
			if mode == "wrong_protocol" {
				p := probe.proxies["candidate"]
				p.Type = "Trojan"
				probe.proxies["candidate"] = p
			}
			m.monitorTraceTransport = func(*url.URL) http.RoundTripper {
				return egressRoundTrip(func(r *http.Request) (*http.Response, error) {
					probe.mu.Lock()
					selected := probe.proxies["probe"].Now
					probe.mu.Unlock()
					if selected != "candidate" {
						t.Fatal("trace used the wrong probe selection")
					}
					if mode == "main_identity_changed" {
						main.base.mu.Lock()
						p := main.base.proxies["candidate"]
						p.Type = "Trojan"
						main.base.proxies["candidate"] = p
						main.base.mu.Unlock()
					}
					return &http.Response{StatusCode: 200, Body: io.NopCloser(strings.NewReader("ip=8.8.8.8\nloc=US\n")), Header: make(http.Header), Request: r}, nil
				})
			}
			value := m.MonitorEgress(context.Background(), "work", model.MonitorNode{Name: "candidate", Protocol: "VLESS"}, func() bool { return true })
			if mode == "ok" {
				if value.Status != "ok" || value.IP != "8.8.8.8" {
					t.Fatal(value)
				}
			} else if value.Status == "ok" || value.IP != "" {
				t.Fatal("unconfirmed isolated evidence accepted", value)
			}
			_, mainWrites := main.base.counts()
			_, probeWrites := probe.counts()
			if mainWrites != 0 {
				t.Fatal("isolated probe wrote to business Controller")
			}
			if mode == "ok" || mode == "main_identity_changed" {
				if probeWrites != 2 || probe.proxies["probe"].Now != "old" {
					t.Fatal("isolated selection not restored", probeWrites)
				}
			} else if probeWrites != 0 {
				t.Fatal("unusable probe Controller mutated")
			}
		})
	}
}
