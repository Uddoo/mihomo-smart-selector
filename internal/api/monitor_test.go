package api

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/base64"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

type monitorController struct{ testController }

func (monitorController) ListProxies(context.Context) (map[string]mihomo.Proxy, error) {
	return map[string]mihomo.Proxy{"ChatGPT": {Name: "ChatGPT", Type: "Selector", Now: "node", All: []string{"node", "DIRECT"}}, "node": {Name: "node", Type: "VLESS"}, "DIRECT": {Name: "DIRECT", Type: "Direct"}}, nil
}

func TestMonitorAPIAuthReadOnlyDiscoveryAndRevision(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP = config.HTTPConfig{Listen: "0.0.0.0:8788", APIToken: "test-token", AllowedCIDRs: []string{"127.0.0.1/32"}}
	cfg.Scanner.ProbeProfiles = append(cfg.Scanner.ProbeProfiles, config.ProbeProfile{ID: "private-monitor", Label: "Private", Probes: []config.Probe{{Name: "probe", URL: "https://private.example/hidden", ExpectedStatus: "200"}}})
	store, err := history.Open(filepath.Join(t.TempDir(), "monitor.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	scanner := scan.NewManager(cfg, monitorController{}, store)
	mon, err := monitor.New(store, scanner)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg.HTTP, scanner, monitorController{})
	if err != nil {
		t.Fatal(err)
	}
	s.WithMonitor(mon)
	call := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:4321"
		if auth {
			r.Header.Set("Authorization", "Bearer test-token")
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	create := `{"revision":0,"enabled":true,"group":"ChatGPT","profile_id":"private-monitor","nodes":["node"]}`
	for _, req := range []struct{ method, path, body string }{{"GET", "/api/v1/monitor", ""}, {"GET", "/api/v1/monitor/catalog?group=ChatGPT&profile_id=private-monitor", ""}, {"PUT", "/api/v1/monitor/plan", create}, {"PUT", "/api/v1/monitor/failover", `{"revision":1,"enabled":true}`}, {"POST", "/api/v1/monitor/retest", `{"node_id":"node","revision":1}`}} {
		if w := call(req.method, req.path, req.body, false); w.Code != 401 {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	for _, path := range []string{"/api/v1/monitor/storage", "/api/v1/monitor/correlations", "/api/v1/monitor/series", "/api/v1/monitor/revisions", "/api/v1/monitor/incidents"} {
		if w := call("GET", path, "", false); w.Code != 401 {
			t.Fatal("analysis bypassed auth", path, w.Code)
		}
	}
	if w := call("POST", "/api/v1/monitor/diagnostics", `{}`, false); w.Code != 401 {
		t.Fatal("diagnostics bypassed auth")
	}
	for _, path := range []string{"/api/v1/monitor", "/api/v1/monitor/catalog?group=ChatGPT&profile_id=private-monitor"} {
		w := call("GET", path, "", true)
		if w.Code != 200 || strings.Contains(w.Body.String(), "private.example") || strings.Contains(w.Body.String(), "DIRECT") {
			t.Fatal(w.Code, w.Body.String())
		}
	}
	if p, _ := store.MonitorPlan(context.Background(), scanner.MonitorScope()); p != nil {
		t.Fatal("GET created monitor plan")
	}
	w := call("PUT", "/api/v1/monitor/plan", create, true)
	if w.Code != 200 {
		t.Fatal(w.Code, w.Body.String())
	}
	var p model.MonitorPlan
	if err = json.Unmarshal(w.Body.Bytes(), &p); err != nil {
		t.Fatal(err)
	}
	if w = call("PUT", "/api/v1/monitor/plan", create, true); w.Code != 409 {
		t.Fatal("stale creation not rejected", w.Code)
	}
	if w = call("POST", "/api/v1/monitor/retest", `{"revision":1,"node_id":"other"}`, true); w.Code != 409 {
		t.Fatal("foreign node accepted")
	}
	if w = call("PUT", "/api/v1/monitor/plan", `{"revision":1,"enabled":false}`, true); w.Code != 200 {
		t.Fatal(w.Body.String())
	}
	if w = call("GET", "/api/v1/monitor", "", true); w.Code != 200 || strings.Contains(w.Body.String(), "private.example") {
		t.Fatal(w.Body.String())
	}
	if w = call("GET", "/api/v1/monitor?window=7d", "", true); w.Code != 200 || !strings.Contains(w.Body.String(), `"window":"7d"`) {
		t.Fatal(w.Body.String())
	}
	if w = call("GET", "/api/v1/monitor?window=30d", "", true); w.Code != 400 {
		t.Fatal("unbounded window accepted")
	}
	if w = call("POST", "/api/v1/monitor/diagnostics", `{}`, true); w.Code != 200 || w.Header().Get("Content-Type") != "application/zip" || !strings.HasPrefix(w.Body.String(), "PK") {
		t.Fatal("not a ZIP", w.Code)
	}
	r := httptest.NewRequest("POST", "/api/v1/monitor/diagnostics", strings.NewReader(`{}`))
	r.RemoteAddr = "127.0.0.1:4321"
	r.Header.Set("Authorization", "Bearer test-token")
	r.Header.Set("Accept", "application/json")
	envelope := httptest.NewRecorder()
	s.Handler().ServeHTTP(envelope, r)
	var payload map[string]string
	if err = json.Unmarshal(envelope.Body.Bytes(), &payload); err != nil {
		t.Fatal(err)
	}
	binary, err := base64.StdEncoding.DecodeString(payload["data"])
	if err != nil {
		t.Fatal(err)
	}
	archive, err := zip.NewReader(bytes.NewReader(binary), int64(len(binary)))
	if err != nil || len(archive.File) != 5 || envelope.Header().Get("Content-Disposition") != "" {
		t.Fatal("browser export envelope invalid", err)
	}
}
