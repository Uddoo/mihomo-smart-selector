package api

import (
	"context"
	"encoding/json"
	"net/http/httptest"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

type summaryController struct{ multiMonitorController }

func (summaryController) ListProviders(context.Context) ([]mihomo.Provider, error) {
	return []mihomo.Provider{{Name: "fixture", Proxies: []mihomo.Proxy{{Name: "member", Type: "VLESS", All: []string{"nested-member"}}}}}, nil
}

func TestSummaryViewsPreserveLegacyDataAndTaskIsolation(t *testing.T) {
	ctx := context.Background()
	cfg := config.Defaults()
	cfg.HTTP = config.HTTPConfig{Listen: "0.0.0.0:8788", APIToken: "fixture-token", AllowedCIDRs: []string{"127.0.0.1/32"}}
	store, err := history.Open(filepath.Join(t.TempDir(), "summary.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	controller := summaryController{}
	scanner := scan.NewManager(cfg, controller, store)
	m, err := monitor.New(store, scanner)
	if err != nil {
		t.Fatal(err)
	}
	s, err := New(cfg.HTTP, scanner, controller)
	if err != nil {
		t.Fatal(err)
	}
	s.WithMonitor(m)
	call := func(path string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest("GET", path, nil)
		r.RemoteAddr = "127.0.0.1:4321"
		if auth {
			r.Header.Set("Authorization", "Bearer fixture-token")
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	for _, view := range []string{"", "?view=full", "?view=summary"} {
		if w := call("/api/v1/providers"+view, false); w.Code != 401 {
			t.Fatal("summary bypassed authentication", w.Code)
		}
		w := call("/api/v1/providers"+view, true)
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		containsMembers := strings.Contains(w.Body.String(), "nested-member")
		if containsMembers == (view == "?view=summary") {
			t.Fatal("provider representation contract", view, w.Body.String())
		}
	}
	if w := call("/api/v1/providers?view=invalid", true); w.Code != 400 {
		t.Fatal("invalid provider view accepted")
	}
	plans := []*model.MonitorPlan{}
	for _, group := range []string{"ChatGPT", "Video"} {
		p, err := m.SaveTask(ctx, "", model.MonitorRequest{Enabled: true, Group: group, ProfileID: "chatgpt", Nodes: []string{"node"}})
		if err != nil {
			t.Fatal(err)
		}
		plans = append(plans, p)
		v := model.MonitorSample{NodeID: p.Nodes[0].ID, Kind: "baseline", Slot: 0, At: time.Now().UTC(), Outcome: "success", DelayMS: 137}
		if _, err := store.RecordMonitor(ctx, p.ID, v, model.MonitorState{Status: "healthy", LastAt: v.At}, nil); err != nil {
			t.Fatal(err)
		}
	}
	path := "/api/v1/monitor/tasks/" + plans[0].TaskID + "/overview?window=24h"
	row := func(w *httptest.ResponseRecorder) map[string]json.RawMessage {
		t.Helper()
		if w.Code != 200 {
			t.Fatal(w.Code, w.Body.String())
		}
		var body struct {
			Rows []map[string]json.RawMessage `json:"rows"`
		}
		if err := json.Unmarshal(w.Body.Bytes(), &body); err != nil || len(body.Rows) != 1 {
			t.Fatal("missing overview row", err)
		}
		return body.Rows[0]
	}
	full := row(call(path, true))
	var samples []model.MonitorSample
	if err := json.Unmarshal(full["series"], &samples); err != nil || len(samples) != 1 {
		t.Fatal("missing legacy samples", err, string(full["series"]))
	}
	for _, suffix := range []string{"&view=summary", "&view=summary&series_id=" + plans[1].Nodes[0].SeriesID} {
		if w := call(path+suffix, false); w.Code != 401 {
			t.Fatal("monitor summary bypassed authentication")
		}
		summary := row(call(path+suffix, true))
		if _, exists := summary["series"]; exists {
			t.Fatal("unrequested/foreign samples leaked")
		}
		if string(summary["metrics"]) != string(full["metrics"]) {
			t.Fatal("summary changed metrics")
		}
	}
	selected := row(call(path+"&view=summary&series_id="+plans[0].Nodes[0].SeriesID, true))
	if string(selected["series"]) != string(full["series"]) {
		t.Fatal("selected evidence changed")
	}
	if string(row(call(path, true))["series"]) != string(full["series"]) {
		t.Fatal("summary mutated the cached full response")
	}
	if w := call(path+"&view=invalid", true); w.Code != 400 {
		t.Fatal("invalid overview view accepted")
	}
	if w := call("/api/v1/monitor?view=summary", true); w.Code != 409 {
		t.Fatal("summary bypassed legacy task ambiguity")
	}
}
