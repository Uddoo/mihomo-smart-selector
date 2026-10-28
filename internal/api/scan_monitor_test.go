package api

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

type scanMonitorController struct {
	testController
	provider string
}

func (*scanMonitorController) ListProxies(context.Context) (map[string]mihomo.Proxy, error) {
	return map[string]mihomo.Proxy{
		"ChatGPT": {Name: "ChatGPT", Type: "Selector", All: []string{"a", "b", "c"}},
		"Video":   {Name: "Video", Type: "Selector", All: []string{"a", "b", "c"}},
		"a":       {Name: "a", Type: "VLESS"},
		"b":       {Name: "b", Type: "VLESS"},
		"c":       {Name: "c", Type: "VLESS"},
	}, nil
}

func (c *scanMonitorController) ListProviders(context.Context) ([]mihomo.Provider, error) {
	return []mihomo.Provider{{Name: c.provider, Proxies: []mihomo.Proxy{{Name: "a"}, {Name: "b"}, {Name: "c"}}}}, nil
}

type scanMonitorFixture struct {
	handler http.Handler
	store   *history.Store
	scanner *scan.Manager
}

func newScanMonitorFixture(t *testing.T) *scanMonitorFixture {
	t.Helper()
	cfg := config.Defaults()
	cfg.HTTP = config.HTTPConfig{Listen: "127.0.0.1:8788", AllowedCIDRs: []string{"127.0.0.1/32"}}
	store, err := history.Open(filepath.Join(t.TempDir(), "monitor.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	controller := &scanMonitorController{provider: "current-provider"}
	scanner := scan.NewManager(cfg, controller, store)
	manager, err := monitor.New(store, scanner)
	if err != nil {
		t.Fatal(err)
	}
	server, err := New(cfg.HTTP, scanner, controller)
	if err != nil {
		t.Fatal(err)
	}
	server.WithMonitor(manager)
	return &scanMonitorFixture{handler: server.Handler(), store: store, scanner: scanner}
}

func (f *scanMonitorFixture) call(t *testing.T, method, path string, body map[string]any) *httptest.ResponseRecorder {
	t.Helper()
	data, err := json.Marshal(body)
	if err != nil {
		t.Fatal(err)
	}
	r := httptest.NewRequest(method, path, bytes.NewReader(data))
	r.RemoteAddr = "127.0.0.1:4321"
	w := httptest.NewRecorder()
	f.handler.ServeHTTP(w, r)
	return w
}

func (f *scanMonitorFixture) sourceScan() model.Scan {
	old := time.Now().UTC().Add(-24 * time.Hour)
	return model.Scan{
		ID: "monitor-source", ControllerScope: f.scanner.MonitorScope(), Status: model.ScanComplete,
		Request:   model.ScanRequest{TargetGroup: "ChatGPT", ProfileID: "chatgpt", Mode: "stable"},
		StartedAt: old, CompletedAt: &old,
		Results: []model.NodeResult{
			{Name: "a", Provider: "current-provider", Rank: 1, Stage: "refined", Score: 50, SuccessRate: 0, MeasuredAt: old, StrictVerificationStatus: "failed"},
			{Name: "b", Provider: "current-provider", Rank: 2, Stage: "screened", Score: 90, SuccessRate: 1, MeasuredAt: old},
			{Name: "c", Provider: "current-provider", Rank: 3, Stage: "refined", Score: 40, SuccessRate: 1, MeasuredAt: old},
		},
	}
}

func (f *scanMonitorFixture) insertScan(t *testing.T, source model.Scan) {
	t.Helper()
	if err := f.store.CreateScan(context.Background(), source); err != nil {
		t.Fatal(err)
	}
	if err := f.store.CompleteScan(context.Background(), source); err != nil {
		t.Fatal(err)
	}
}

func requireScanMonitorPlan(t *testing.T, w *httptest.ResponseRecorder, status int, names []string) model.MonitorPlan {
	t.Helper()
	var plan model.MonitorPlan
	if w.Code != status || json.Unmarshal(w.Body.Bytes(), &plan) != nil || plan.TaskID == "" {
		t.Fatalf("save status = %d, body = %s", w.Code, w.Body.String())
	}
	actual := make([]string, len(plan.Nodes))
	for i, node := range plan.Nodes {
		actual[i] = node.Name
	}
	if !reflect.DeepEqual(actual, names) {
		t.Fatalf("saved nodes = %v, want %v", actual, names)
	}
	return plan
}

func TestMonitorTasksScanIDSaveAndLegacyRequests(t *testing.T) {
	f := newScanMonitorFixture(t)
	source := f.sourceScan()
	f.insertScan(t, source)
	body := map[string]any{
		"scan_id": source.ID, "enabled": true, "group": "ChatGPT", "profile_id": "chatgpt", "nodes": []string{"a", "c"},
	}
	created := requireScanMonitorPlan(t, f.call(t, "POST", "/api/v1/monitor/tasks", body), 201, []string{"a", "c"})
	if created.Revision != 1 {
		t.Fatal("create revision", created.Revision)
	}
	body["revision"] = created.Revision
	body["nodes"] = []string{"a", "c", "b"}
	updated := requireScanMonitorPlan(t, f.call(t, "PUT", "/api/v1/monitor/tasks/"+created.TaskID, body), 200, []string{"a", "c", "b"})
	if updated.Revision != 2 || updated.TaskID != created.TaskID {
		t.Fatal("update identity or revision", updated)
	}
	delete(body, "scan_id")
	body["revision"] = updated.Revision
	body["nodes"] = []string{"b"}
	legacy := requireScanMonitorPlan(t, f.call(t, "PUT", "/api/v1/monitor/tasks/"+created.TaskID, body), 200, []string{"b"})
	if legacy.Revision != 3 {
		t.Fatal("legacy update revision", legacy.Revision)
	}
	body["scan_id"] = ""
	body["revision"] = legacy.Revision
	body["nodes"] = []string{"c"}
	requireScanMonitorPlan(t, f.call(t, "PUT", "/api/v1/monitor/tasks/"+created.TaskID, body), 200, []string{"c"})
}

func TestMonitorTasksScanIDRejectsWithoutSaving(t *testing.T) {
	tests := []struct {
		name   string
		change func(*model.Scan, map[string]any)
		want   string
	}{
		{name: "foreign controller", want: "扫描不属于当前 Controller，请重新扫描", change: func(s *model.Scan, _ map[string]any) {
			s.ControllerScope = "http://previous-controller.example:9090"
		}},
		{name: "empty scope", want: "扫描不属于当前 Controller，请重新扫描", change: func(s *model.Scan, _ map[string]any) {
			s.ControllerScope = ""
		}},
		{name: "provider changed", want: "排名中的节点已失效，请重新扫描后替换。", change: func(s *model.Scan, _ map[string]any) {
			s.Results[0].Provider = "previous-provider"
		}},
		{name: "empty provider changed", want: "排名中的节点已失效，请重新扫描后替换。", change: func(s *model.Scan, _ map[string]any) {
			s.Results[2].Provider = ""
		}},
		{name: "group tampered", want: "扫描排名与提交名单不一致，请重新读取扫描。", change: func(_ *model.Scan, body map[string]any) {
			body["group"] = "Video"
		}},
		{name: "ranking tampered", want: "扫描排名与提交名单不一致，请重新读取扫描。", change: func(_ *model.Scan, body map[string]any) {
			body["nodes"] = []string{"c", "a"}
		}},
		{name: "empty nodes", want: "扫描排名与提交名单不一致，请重新读取扫描。", change: func(_ *model.Scan, body map[string]any) {
			body["nodes"] = []string{}
		}},
		{name: "scan running", want: "扫描完成后可替换监控列表。", change: func(s *model.Scan, _ map[string]any) {
			s.Status = model.ScanRunning
		}},
		{name: "scan failed", want: "扫描完成后可替换监控列表。", change: func(s *model.Scan, _ map[string]any) {
			s.Status = model.ScanFailed
		}},
		{name: "unknown scan", want: `scan "absent" not found`, change: func(_ *model.Scan, body map[string]any) {
			body["scan_id"] = "absent"
		}},
	}
	for _, tt := range tests {
		for _, method := range []string{"POST", "PUT"} {
			t.Run(tt.name+"/"+method, func(t *testing.T) {
				ctx := context.Background()
				f := newScanMonitorFixture(t)
				source := f.sourceScan()
				body := map[string]any{
					"scan_id": source.ID, "enabled": true, "group": "ChatGPT", "profile_id": "chatgpt", "nodes": []string{"a", "c"},
				}
				path := "/api/v1/monitor/tasks"
				var taskID string
				if method == "PUT" {
					created := requireScanMonitorPlan(t, f.call(t, "POST", path, map[string]any{
						"enabled": true, "group": "ChatGPT", "profile_id": "chatgpt", "nodes": []string{"b"},
					}), 201, []string{"b"})
					taskID = created.TaskID
					path += "/" + taskID
					body["revision"] = created.Revision
				}
				tt.change(&source, body)
				f.insertScan(t, source)
				before, err := f.store.MonitorTasks(ctx, f.scanner.MonitorScope())
				if err != nil {
					t.Fatal(err)
				}
				w := f.call(t, method, path, body)
				var response struct {
					Error string `json:"error"`
				}
				if w.Code != 409 || json.Unmarshal(w.Body.Bytes(), &response) != nil || response.Error != tt.want {
					t.Fatalf("rejection status = %d, body = %s, want %q", w.Code, w.Body.String(), tt.want)
				}
				after, err := f.store.MonitorTasks(ctx, f.scanner.MonitorScope())
				if err != nil || !reflect.DeepEqual(before, after) {
					t.Fatalf("rejection mutated tasks, revisions or nodes: before=%+v after=%+v err=%v", before, after, err)
				}
				if taskID != "" {
					revisions, err := f.store.MonitorTaskRevisions(ctx, f.scanner.MonitorScope(), taskID)
					if err != nil || len(revisions) != 1 {
						t.Fatalf("rejection added a revision: count=%d err=%v", len(revisions), err)
					}
				}
			})
		}
	}
}
