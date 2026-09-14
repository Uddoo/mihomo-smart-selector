package scan

import (
	"bytes"
	"context"
	"encoding/json"
	"io"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"strings"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type verificationFaultController struct {
	base       *probeTargetController
	mode       string
	attempted  atomic.Bool
	restored   atomic.Bool
	probeCalls atomic.Int32
	afterProbe atomic.Int32
	cancel     context.CancelFunc
}

func (f *verificationFaultController) change(change func(*mihomo.Proxy)) {
	f.base.mu.Lock()
	defer f.base.mu.Unlock()
	p := f.base.proxies["probe"]
	change(&p)
	f.base.proxies["probe"] = p
}

func (f *verificationFaultController) current() string {
	f.base.mu.Lock()
	defer f.base.mu.Unlock()
	return f.base.proxies["probe"].Now
}

func (f *verificationFaultController) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method == http.MethodGet && r.URL.Path == "/proxies" {
		if f.probeCalls.Load() > 0 && f.mode == "restore_conflict" && f.afterProbe.Add(1) == 2 {
			f.change(func(p *mihomo.Proxy) { p.Now = "external" })
		}
		if f.mode == "initial_read_error" || f.mode == "switch_read_error" && f.attempted.Load() || f.mode == "post_read_error" && f.probeCalls.Load() > 0 || f.mode == "restore_read_error" && f.restored.Load() {
			http.Error(w, "injected read failure", http.StatusServiceUnavailable)
			return
		}
	}
	if r.Method == http.MethodPut {
		body, _ := io.ReadAll(r.Body)
		r.Body = io.NopCloser(bytes.NewReader(body))
		var payload struct{ Name string }
		_ = json.Unmarshal(body, &payload)
		isRestore := payload.Name == "old"
		if isRestore {
			f.restored.Store(true)
		} else {
			f.attempted.Store(true)
		}
		fault := ""
		if !isRestore && (f.mode == "switch_noop" || f.mode == "switch_error" || f.mode == "switch_error_applied") {
			fault = f.mode
		}
		if isRestore && (f.mode == "restore_noop" || f.mode == "restore_error" || f.mode == "restore_error_applied" || f.mode == "cancel_restore_error") {
			fault = f.mode
		}
		if fault != "" {
			f.base.mu.Lock()
			f.base.writes++
			f.base.mu.Unlock()
			if strings.HasSuffix(fault, "_applied") {
				f.change(func(p *mihomo.Proxy) { p.Now = payload.Name })
			}
			if strings.HasSuffix(fault, "noop") {
				w.WriteHeader(http.StatusNoContent)
			} else {
				http.Error(w, "injected write failure", http.StatusInternalServerError)
			}
			return
		}
	}
	f.base.ServeHTTP(w, r)
}

func newVerificationFixture(t *testing.T, phase, mode string) (*Manager, *verificationFaultController, config.ProbeProfile) {
	t.Helper()
	f := &verificationFaultController{mode: mode, base: &probeTargetController{proxies: map[string]mihomo.Proxy{
		"probe":     {Name: "probe", Type: "Selector", Now: "old", All: []string{"old", "candidate", "external"}},
		"work":      {Name: "work", Type: "Selector", Now: "candidate", All: []string{"candidate"}},
		"candidate": {Name: "candidate", Type: "VLESS"},
	}}}
	controller := httptest.NewServer(f)
	t.Cleanup(controller.Close)
	proxy := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		count := f.probeCalls.Add(1)
		region := "JP"
		if f.current() == "candidate" {
			region = "US"
		}
		switch mode {
		case "during_probe_change":
			f.change(func(p *mihomo.Proxy) { p.Now = "external" })
		case "second_probe_change":
			if count == 2 {
				f.change(func(p *mihomo.Proxy) { p.Now = "external" })
			}
		case "member_removed":
			f.change(func(p *mihomo.Proxy) { p.All = []string{"old", "external"} })
		case "type_changed":
			f.change(func(p *mihomo.Proxy) { p.Type = "URLTest" })
		case "cancel", "cancel_restore_error":
			f.cancel()
		}
		_, _ = w.Write([]byte("loc=" + region + "\nproof\n"))
	}))
	t.Cleanup(proxy.Close)
	cfg := config.Defaults()
	cfg.Mihomo.Controller = controller.URL
	cfg.Scanner.StrictVerification = config.StrictVerificationConfig{Enabled: phase != "egress", SelectorGroup: "probe", ProxyURL: proxy.URL, MaxCandidates: 5}
	cfg.EgressVerification = config.EgressConfig{Enabled: phase != "strict", SelectorGroup: "probe", ProxyURL: proxy.URL, TraceURL: "http://trace.invalid/"}
	profile := config.ProbeProfile{ID: "safety", Label: "Safety fixture", Probes: []config.Probe{{Name: "latency", URL: "https://example.com/", ExpectedStatus: "200"}}, StrictProbes: []config.StrictProbe{{Name: "proof", URL: "http://service.invalid/", ExpectedStatus: "200", BodyContains: "proof"}}}
	if mode == "second_probe_change" {
		profile.StrictProbes = append(profile.StrictProbes, profile.StrictProbes[0])
	}
	cfg.Scanner.ProbeProfiles = append(cfg.Scanner.ProbeProfiles, profile)
	client, err := mihomo.NewWithSecret(cfg.Mihomo, "")
	if err != nil {
		t.Fatal(err)
	}
	return NewManager(cfg, client, nil), f, profile
}

func TestVerificationConfirmsIdentityAndReportsRestore(t *testing.T) {
	for _, phase := range []string{"egress", "strict"} {
		for _, tc := range []struct {
			mode, warning, current string
			calls                  int
			verified               bool
		}{
			{"normal", "", "old", 1, true},
			{"initial_read_error", "", "old", 0, false},
			{"switch_noop", "", "old", 0, false},
			{"switch_error", "", "old", 0, false},
			{"switch_error_applied", "", "old", 0, false},
			{"switch_read_error", "probe_restore_unconfirmed", "candidate", 0, false},
			{"post_read_error", "probe_restore_unconfirmed", "candidate", 1, false},
			{"during_probe_change", "probe_restore_conflict", "external", 1, false},
			{"member_removed", "", "old", 1, false},
			{"type_changed", "probe_restore_unconfirmed", "candidate", 1, false},
			{"restore_error", "probe_restore_failed", "candidate", 1, true},
			{"restore_noop", "probe_restore_failed", "candidate", 1, true},
			{"restore_error_applied", "", "old", 1, true},
			{"restore_read_error", "probe_restore_unconfirmed", "old", 1, true},
			{"restore_conflict", "probe_restore_conflict", "external", 1, true},
			{"cancel", "", "old", 1, false},
			{"cancel_restore_error", "probe_restore_failed", "candidate", 1, false},
		} {
			t.Run(phase+"/"+tc.mode, func(t *testing.T) {
				m, f, profile := newVerificationFixture(t, phase, tc.mode)
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				f.cancel = cancel
				results := []model.NodeResult{{Name: "candidate", VerifiedRegion: "DE", StrictVerificationStatus: "passed", StrictChecks: []model.StrictCheck{{Status: "passed"}}}}
				var warning *model.ScanWarning
				if phase == "egress" {
					warning = m.verifyEgress(ctx, results, "test")
				} else {
					warning = m.verifyStrict(ctx, profile, results, "test")
				}
				r := results[0]
				verified := r.VerifiedRegion != ""
				if phase == "strict" {
					verified = r.StrictVerificationStatus == "passed"
				}
				if verified != tc.verified || int(f.probeCalls.Load()) != tc.calls || f.current() != tc.current {
					t.Fatalf("result=%+v HTTP=%d current=%s; want verified=%v HTTP=%d current=%s", r, f.probeCalls.Load(), f.current(), tc.verified, tc.calls, tc.current)
				}
				if !verified && phase == "strict" && (len(r.StrictChecks) != 0 || r.RestrictionStatus != "not_checked") {
					t.Fatal("unconfirmed strict response leaked evidence")
				}
				if !verified && phase == "egress" && r.EgressError == "" {
					t.Fatal("unconfirmed egress response omitted its reason")
				}
				if tc.warning == "" && warning != nil || tc.warning != "" && (warning == nil || warning.Code != tc.warning || warning.Group != "probe" || warning.Phase != phase || warning.Message == "") {
					t.Fatalf("warning=%+v, want %q", warning, tc.warning)
				}
			})
		}
	}
}

func TestVerificationRejectsUnsafeInitialStateAndUsesFreshBaseline(t *testing.T) {
	for _, phase := range []string{"egress", "strict"} {
		for _, state := range []string{"empty", "missing_member", "missing_candidate", "wrong_type", "changed_baseline"} {
			t.Run(phase+"/"+state, func(t *testing.T) {
				m, f, profile := newVerificationFixture(t, phase, "normal")
				f.change(func(p *mihomo.Proxy) {
					switch state {
					case "empty":
						p.Now = ""
					case "missing_member":
						p.Now = "removed"
					case "missing_candidate":
						p.All = []string{"old", "external"}
					case "wrong_type":
						p.Type = "URLTest"
					default:
						p.Now = "external"
					}
				})
				results := []model.NodeResult{{Name: "candidate"}}
				if phase == "egress" {
					m.verifyEgress(context.Background(), results, "test")
				} else {
					m.verifyStrict(context.Background(), profile, results, "test")
				}
				_, writes := f.base.counts()
				if state == "changed_baseline" {
					if f.current() != "external" || writes != 2 {
						t.Fatal("restoration did not use the fresh pre-phase selection")
					}
				} else if writes != 0 || f.probeCalls.Load() != 0 {
					t.Fatal("unrestorable initial state was mutated")
				}
			})
		}
	}
}

func TestVerificationSkipsMissingMembersButStopsAfterUnconfirmedSwitch(t *testing.T) {
	for _, phase := range []string{"egress", "strict"} {
		for _, mode := range []string{"normal", "switch_noop"} {
			t.Run(phase+"/"+mode, func(t *testing.T) {
				m, f, profile := newVerificationFixture(t, phase, mode)
				first := "missing"
				if mode == "switch_noop" {
					first = "candidate"
				}
				results := []model.NodeResult{{Name: first, Score: 90}, {Name: "candidate", Score: 80}}
				if phase == "egress" {
					m.verifyEgress(context.Background(), results, "test")
				} else {
					m.verifyStrict(context.Background(), profile, results, "test")
				}
				if mode == "normal" {
					if f.probeCalls.Load() != 1 || (phase == "egress" && results[1].VerifiedRegion != "US") || (phase == "strict" && results[1].StrictVerificationStatus != "passed") {
						t.Fatal("missing member prevented another eligible candidate from being verified")
					}
				} else {
					_, writes := f.base.counts()
					if f.probeCalls.Load() != 0 || writes != 1 || results[1].VerifiedRegion != "" || results[1].StrictVerificationStatus == "passed" {
						t.Fatal("unconfirmed switch did not stop remaining candidates")
					}
				}
			})
		}
	}

}

func TestStrictDiscardsEarlierChecksAfterLaterIdentityChange(t *testing.T) {
	m, f, profile := newVerificationFixture(t, "strict", "second_probe_change")
	results := []model.NodeResult{{Name: "candidate"}}
	warning := m.verifyStrict(context.Background(), profile, results, "test")
	if f.probeCalls.Load() != 2 || results[0].StrictVerificationStatus != "probe_selector_changed" || len(results[0].StrictChecks) != 0 || warning == nil || warning.Code != "probe_restore_conflict" {
		t.Fatalf("partial evidence was retained: results=%+v warning=%+v", results, warning)
	}
}

func TestVerificationWarningsPersistThroughScanCompletionAndCancellation(t *testing.T) {
	for _, phase := range []string{"egress", "strict", "both"} {
		for _, cancelled := range []bool{false, true} {
			name := phase + "/completed"
			mode := "restore_error"
			if cancelled {
				name = phase + "/cancelled"
				mode = "cancel_restore_error"
			}
			t.Run(name, func(t *testing.T) {
				m, f, profile := newVerificationFixture(t, phase, mode)
				path := filepath.Join(t.TempDir(), "warnings.db")
				store, err := history.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				defer func() { _ = store.Close() }()
				m.store = store
				ctx, cancel := context.WithCancel(context.Background())
				defer cancel()
				f.cancel = cancel
				s := model.Scan{ID: "warning-scan", ControllerScope: m.bindingScope(), Status: model.ScanRunning, StartedAt: time.Now().UTC(), Request: model.ScanRequest{TargetGroup: "work", ProfileID: profile.ID, Mode: "quick"}}
				if err := store.CreateScan(ctx, s); err != nil {
					t.Fatal(err)
				}
				m.setActive(s)
				events, unsubscribe := m.Subscribe(s.ID)
				m.run(ctx, s)
				unsubscribe()
				found := false
				for event := range events {
					found = found || event.Kind == "warning"
				}
				if !found {
					t.Fatal("cleanup warning was not published")
				}
				got, err := m.Get(context.Background(), s.ID)
				status := model.ScanComplete
				if cancelled {
					status = model.ScanCancelled
				}
				if err != nil || got.Status != status || len(got.Warnings) != 1 || got.Warnings[0].Code != "probe_restore_failed" {
					t.Fatalf("scan=%+v err=%v", got, err)
				}
				if phase == "both" && (f.probeCalls.Load() != 1 || got.Results[0].StrictVerificationStatus != "probe_verification_stopped") {
					t.Fatal("strict verification reused a group with unresolved restoration")
				}
				if err := store.Close(); err != nil {
					t.Fatal(err)
				}
				store, err = history.Open(path)
				if err != nil {
					t.Fatal(err)
				}
				reloaded := NewManager(m.currentConfig(), m.client, store)
				restored, err := reloaded.Get(context.Background(), s.ID)
				if err != nil || !reflect.DeepEqual(restored.Warnings, got.Warnings) || restored.Status != status {
					t.Fatalf("warning lost on reopen: %+v %v", restored, err)
				}
			})
		}
	}
}
