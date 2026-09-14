package scan

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Count actual HTTP writes so rejection tests cover the client and SQLite
// boundaries, not only the displayed eligibility reason.
type probeTargetController struct {
	mu      sync.Mutex
	proxies map[string]mihomo.Proxy
	reads   int
	writes  int
}

func (c *probeTargetController) counts() (int, int) {
	c.mu.Lock()
	defer c.mu.Unlock()
	return c.reads, c.writes
}

func (c *probeTargetController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	c.mu.Lock()
	defer c.mu.Unlock()
	w.Header().Set("Content-Type", "application/json")
	switch {
	case r.Method == http.MethodGet && r.URL.Path == "/proxies":
		c.reads++
		_ = json.NewEncoder(w).Encode(map[string]any{"proxies": c.proxies})
	case r.Method == http.MethodGet && r.URL.Path == "/providers/proxies":
		_, _ = w.Write([]byte(`{"providers":{}}`))
	case r.Method == http.MethodGet && strings.HasSuffix(r.URL.Path, "/delay"):
		_, _ = w.Write([]byte(`{"delay":25}`))
	case r.Method == http.MethodPut && strings.HasPrefix(r.URL.Path, "/proxies/"):
		c.writes++
		var payload struct {
			Name string `json:"name"`
		}
		if err := json.NewDecoder(r.Body).Decode(&payload); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		name := strings.TrimPrefix(r.URL.Path, "/proxies/")
		p := c.proxies[name]
		p.Now = payload.Name
		c.proxies[name] = p
		w.WriteHeader(http.StatusNoContent)
	default:
		http.Error(w, "unexpected fixture request", http.StatusNotFound)
	}
}

func newProbeTargetManager(t *testing.T) (*Manager, *probeTargetController, *history.Store) {
	t.Helper()
	c := &probeTargetController{proxies: map[string]mihomo.Proxy{
		"probe":     {Name: "probe", Type: "Selector", Now: "old", All: []string{"candidate", "old"}},
		"work":      {Name: "work", Type: "Selector", Now: "old", All: []string{"candidate", "old"}},
		"candidate": {Name: "candidate", Type: "VLESS"},
		"old":       {Name: "old", Type: "VLESS"},
	}}
	controller := httptest.NewServer(c)
	t.Cleanup(controller.Close)
	cfg := config.Defaults()
	cfg.Mihomo.Controller = controller.URL
	// Verification traffic, if a scan is incorrectly admitted, remains local.
	cfg.Scanner.StrictVerification = config.StrictVerificationConfig{SelectorGroup: "probe", ProxyURL: controller.URL, MaxCandidates: 5}
	cfg.EgressVerification = config.EgressConfig{SelectorGroup: "probe", ProxyURL: controller.URL, TraceURL: "https://example.com/trace"}
	client, err := mihomo.NewWithSecret(cfg.Mihomo, "")
	if err != nil {
		t.Fatal(err)
	}
	store, err := history.Open(filepath.Join(t.TempDir(), "probe-target.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _ = store.Close() })
	m := NewManager(cfg, client, store)
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := m.Shutdown(ctx); err != nil {
			t.Error(err)
		}
	})
	return m, c, store
}

func reserveProbeTarget(t *testing.T, m *Manager, mode, group string) {
	t.Helper()
	s := m.Settings()
	s.Strict.Enabled = mode == "strict" || mode == "both"
	s.Egress.Enabled = mode == "egress" || mode == "both"
	s.Strict.SelectorGroup = group
	s.Egress.SelectorGroup = group
	if _, err := m.SaveSettings(context.Background(), s); err != nil {
		t.Fatal(err)
	}
}

func completedProbeTargetScan(t *testing.T, m *Manager, store *history.Store) model.Scan {
	t.Helper()
	now := time.Now().UTC()
	s := model.Scan{ID: "historical", ControllerScope: m.bindingScope(), Status: model.ScanComplete, StartedAt: now, CompletedAt: &now,
		Request: model.ScanRequest{TargetGroup: "work", ProfileID: "internet-baseline", Mode: "stable"},
		Results: []model.NodeResult{{Name: "candidate", Rank: 1, Stage: "refined", SuccessRate: 1, MeasuredAt: now}}}
	if err := store.CreateScan(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteScan(context.Background(), s); err != nil {
		t.Fatal(err)
	}
	return s
}

func TestProbeTargetRejectsNormalizedName(t *testing.T) {
	for _, mode := range []string{"strict", "egress", "both"} {
		for _, input := range []struct{ name, target string }{{"exact", "probe"}, {"ascii", " \tprobe\r\n"}, {"unicode", "\u3000probe\u00a0"}} {
			for _, entry := range []string{"preflight", "start"} {
				t.Run(mode+"/"+input.name+"/"+entry, func(t *testing.T) {
					m, c, _ := newProbeTargetManager(t)
					reserveProbeTarget(t, m, mode, "probe")
					req := model.ScanRequest{TargetGroup: input.target, ProfileID: "internet-baseline", Mode: "quick"}
					var err error
					if entry == "preflight" {
						_, err = m.Preflight(context.Background(), req)
					} else {
						_, err = m.Start(req)
					}
					if err == nil || !strings.Contains(err.Error(), "专用探测组") {
						t.Fatalf("reserved target %q: %v", input.target, err)
					}
					if _, writes := c.counts(); writes != 0 || m.hasRunningScan() {
						t.Fatal("rejected target started a scan or changed the Controller")
					}
				})
			}
		}
	}
}

func TestProbeTargetAllowsUnreservedScans(t *testing.T) {
	for _, mode := range []string{"disabled", "strict", "egress", "both"} {
		t.Run(mode, func(t *testing.T) {
			m, _, _ := newProbeTargetManager(t)
			reserveProbeTarget(t, m, mode, "probe")
			target := " work "
			if mode == "disabled" {
				target = " probe "
			}
			req := model.ScanRequest{TargetGroup: target, ProfileID: "internet-baseline", Mode: "quick"}
			if preview, err := m.Preflight(context.Background(), req); err != nil || !preview.Ready {
				t.Fatalf("unreserved preflight: %+v, %v", preview, err)
			}
			if s, err := m.Start(req); err != nil || s.Request.TargetGroup != strings.TrimSpace(target) {
				t.Fatalf("unreserved scan: %+v, %v", s, err)
			}
		})
	}
}

func TestProbeTargetReservationBlocksHistoricalSelection(t *testing.T) {
	for _, mode := range []string{"strict", "egress", "both"} {
		t.Run(mode, func(t *testing.T) {
			ctx := context.Background()
			m, c, store := newProbeTargetManager(t)
			s := completedProbeTargetScan(t, m, store)
			reserveProbeTarget(t, m, mode, "work")
			got, err := m.Get(ctx, s.ID)
			if err != nil || len(got.Results) != 1 {
				t.Fatalf("historical scan: %+v, %v", got, err)
			}
			if !strings.Contains(got.Results[0].SelectionReason, "专用探测组") {
				t.Error("historical scan does not explain the reserved target")
			}
			if _, err := m.SelectRequest(ctx, s.ID, "candidate", "new-selection"); err == nil || !strings.Contains(err.Error(), "专用探测组") {
				t.Errorf("reserved selection: %v", err)
			}
			audits, err := store.ListSwitches(ctx, 10)
			_, writes := c.counts()
			if err != nil || writes != 0 || len(audits) != 0 {
				t.Fatalf("rejected selection: PUTs=%d audits=%+v err=%v", writes, audits, err)
			}
			// Turning verification off releases the group for the same fresh scan.
			reserveProbeTarget(t, m, "disabled", "work")
			got, err = m.Get(ctx, s.ID)
			if err != nil || got.Results[0].SelectionReason != "" {
				t.Fatalf("released scan: %+v, %v", got, err)
			}
			if event, err := m.SelectRequest(ctx, s.ID, "candidate", "new-selection"); err != nil || event.Status != "confirmed" {
				t.Fatalf("released selection: %+v, %v", event, err)
			}
		})
	}
}

func TestProbeTargetReservationPreservesRequestReplay(t *testing.T) {
	for _, mode := range []string{"strict", "egress", "both"} {
		for _, status := range []string{"confirmed", "pending", "unknown", "failed"} {
			t.Run(mode+"/"+status, func(t *testing.T) {
				ctx := context.Background()
				m, c, store := newProbeTargetManager(t)
				s := completedProbeTargetScan(t, m, store)
				if status == "confirmed" {
					if event, err := m.SelectRequest(ctx, s.ID, "candidate", "existing"); err != nil || event.Status != status {
						t.Fatalf("original selection: %+v, %v", event, err)
					}
				} else {
					// Seed durable evidence at other possible points in the switch lifecycle.
					if _, err := store.RecordSwitch(ctx, model.SwitchEvent{ControllerScope: m.bindingScope(), ScanID: s.ID, Group: "work", Previous: "old", Selected: "candidate", Reason: "retained switch evidence", CreatedAt: time.Now().UTC(), Status: status, RequestID: "existing"}); err != nil {
						t.Fatal(err)
					}
				}
				original, err := store.SwitchByRequest(ctx, "existing")
				if err != nil {
					t.Fatal(err)
				}
				reserveProbeTarget(t, m, mode, "work")
				readsBefore, writesBefore := c.counts()
				for _, node := range []string{"candidate", ""} {
					if replay, err := m.SelectRequest(ctx, s.ID, node, "existing"); err != nil || !reflect.DeepEqual(replay, original) {
						t.Fatalf("replay changed persisted evidence: %+v, %v; want %+v", replay, err, original)
					}
				}
				for _, request := range []struct{ scanID, node string }{{s.ID, "old"}, {"another-scan", "candidate"}} {
					if _, err := m.SelectRequest(ctx, request.scanID, request.node, "existing"); err == nil || !strings.Contains(err.Error(), "different selection") {
						t.Fatalf("request ownership was not enforced: %v", err)
					}
				}
				if _, err := m.SelectRequest(ctx, s.ID, "candidate", "new-request"); err == nil || !strings.Contains(err.Error(), "专用探测组") {
					t.Fatalf("new request did not enforce current reservation: %v", err)
				}
				audits, err := store.ListSwitches(ctx, 10)
				reads, writes := c.counts()
				if err != nil || len(audits) != 1 || !reflect.DeepEqual(audits[0], original) || reads != readsBefore || writes != writesBefore {
					t.Fatalf("replay or rejected request changed evidence/Controller: reads=%d/%d PUTs=%d/%d audits=%+v err=%v", reads, readsBefore, writes, writesBefore, audits, err)
				}
			})
		}
	}
}
