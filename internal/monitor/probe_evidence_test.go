package monitor

import (
	"context"
	"errors"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor/probe"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

// Retain the transport cause while replacing every word seen by the caller.
type renamedProbeError struct{ error }

func (renamedProbeError) Error() string   { return "探测请求未完成" }
func (e renamedProbeError) Unwrap() error { return e.error }

type probeErrorClient struct {
	mihomo.Client
	err     error
	offline bool
	checks  int
}

func (c *probeErrorClient) Delay(context.Context, string, string, config.Probe, int) (int, error) {
	return 0, c.err
}
func (c *probeErrorClient) Reachable(context.Context) (string, error) {
	c.checks++
	if c.offline {
		return "", errors.New("version unavailable")
	}
	return "test", nil
}

func TestMonitorProbeEvidenceAndFailoverActions(t *testing.T) {
	for _, tc := range []struct {
		name, node            string
		status                int
		cause                 error
		mask, offline, cancel bool
		outcome, code         string
		checks                int
	}{
		{name: "busy", cause: probe.ErrBusy, mask: true, outcome: "unknown", code: "busy"},
		{name: "401 with different wording", status: 401, mask: true, outcome: "unknown", code: "controller"},
		{name: "403 with different wording", status: 403, mask: true, outcome: "unknown", code: "controller"},
		{name: "node failure", status: 504, outcome: "failure", code: "node_path", checks: 1},
		{name: "HTTP 401 in node name", node: "node HTTP 401", status: 504, outcome: "failure", code: "node_path", checks: 1},
		{name: "controller unreachable", cause: errors.New("dial failed"), offline: true, outcome: "unknown", code: "controller", checks: 1},
		{name: "probe timeout while caller live", cause: context.DeadlineExceeded, outcome: "failure", code: "node_path", checks: 1},
		{name: "cancelled cause", cause: context.Canceled, mask: true},
		{name: "caller cancelled during probe", status: 504, cancel: true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			cause := tc.cause
			if tc.status != 0 {
				server := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { w.WriteHeader(tc.status) }))
				defer server.Close()
				client, err := mihomo.NewWithSecret(config.MihomoConfig{Controller: server.URL, RequestTimeoutSeconds: 2}, "")
				if err != nil {
					t.Fatal(err)
				}
				defer client.CloseIdleConnections()
				_, cause = client.Delay(context.Background(), tc.node, "", config.Probe{}, 200)
				if cause == nil {
					t.Fatal("expected HTTP error")
				}
			}
			if tc.mask {
				cause = renamedProbeError{cause}
			}
			for _, kind := range []string{"baseline", "failover"} {
				t.Run(kind, func(t *testing.T) {
					var m *Manager
					var f *fakeSource
					var switches *fakeSwitchSource
					var now *time.Time
					node := "node-a"
					if kind == "failover" {
						m, switches, now = failoverFixture(t)
						f = switches.fakeSource
						node = "node-b"
					} else {
						m, _, f, now = setup(t)
					}
					ctx, cancel := context.WithCancel(context.Background())
					defer cancel()
					client := &probeErrorClient{err: cause, offline: tc.offline}
					adapter := scan.NewManager(config.Defaults(), client, nil)
					f.delay = func(ctx context.Context) (int, error) {
						if f.calls > 1 {
							return 150, nil
						} // next ranked candidate
						if tc.cancel {
							cancel()
						}
						return adapter.MonitorDelay(ctx, model.MonitorNode{ID: node, Name: tc.node}, config.Probe{})
					}
					if kind == "baseline" {
						m.step(ctx, *now)
					} else {
						m.failover(ctx, *now)
					}
					if client.checks != tc.checks {
						t.Errorf("controller reachability checks=%d, want %d", client.checks, tc.checks)
					}
					samples, err := m.store.MonitorSamples(context.Background(), m.plan.ID, now.Add(-time.Second))
					if err != nil {
						t.Fatal(err)
					}
					var evidence []model.MonitorSample
					for _, s := range samples {
						if s.Kind == kind && s.NodeID == node {
							evidence = append(evidence, s)
						}
					}
					if tc.outcome == "" {
						if len(evidence) != 0 {
							t.Errorf("cancellation wrote evidence: %+v", evidence)
						}
					} else if len(evidence) != 1 || evidence[0].Outcome != tc.outcome || evidence[0].ReasonCode != tc.code {
						t.Errorf("evidence=%+v, want %s/%s", evidence, tc.outcome, tc.code)
					}
					if switches != nil {
						if tc.outcome == "failure" {
							if len(switches.switches) != 1 || switches.switches[0] != "C" || f.calls != 2 {
								t.Errorf("expected verified fallback, got %v after %d probes", switches.switches, f.calls)
							}
						} else if len(switches.switches) != 0 || f.calls != 1 {
							t.Errorf("unexpected fallback: switches=%v probes=%d", switches.switches, f.calls)
						}
					}
				})
			}
		})
	}
}
