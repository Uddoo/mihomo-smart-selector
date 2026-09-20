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

type multiMonitorController struct{ monitorController }

func (c multiMonitorController) ListProxies(ctx context.Context) (map[string]mihomo.Proxy, error) {
	proxies, err := c.monitorController.ListProxies(ctx)
	proxies["Video"] = mihomo.Proxy{Name: "Video", Type: "Selector", Now: "node", All: []string{"node"}}
	return proxies, err
}

func TestMonitorTasksHTTPContractAndLegacyAmbiguity(t *testing.T) {
	cfg := config.Defaults()
	cfg.HTTP = config.HTTPConfig{Listen: "0.0.0.0:8788", APIToken: "fixture-token", AllowedCIDRs: []string{"127.0.0.1/32"}}
	store, err := history.Open(filepath.Join(t.TempDir(), "tasks.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	controller := multiMonitorController{}
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
	call := func(method, path, body string, auth bool) *httptest.ResponseRecorder {
		r := httptest.NewRequest(method, path, strings.NewReader(body))
		r.RemoteAddr = "127.0.0.1:4321"
		if auth {
			r.Header.Set("Authorization", "Bearer fixture-token")
		}
		w := httptest.NewRecorder()
		s.Handler().ServeHTTP(w, r)
		return w
	}
	decodePlan := func(w *httptest.ResponseRecorder, code int) model.MonitorPlan {
		t.Helper()
		var p model.MonitorPlan
		if w.Code != code || json.Unmarshal(w.Body.Bytes(), &p) != nil || p.TaskID == "" {
			t.Fatal(w.Code, w.Body.String())
		}
		return p
	}
	for _, method := range []string{"GET", "POST"} {
		if w := call(method, "/api/v1/monitor/tasks", `{}`, false); w.Code != 401 {
			t.Fatal("auth", w.Code, w.Body.String())
		}
	}
	if w := call("GET", "/api/v1/monitor/tasks", "", true); w.Code != 200 || strings.TrimSpace(w.Body.String()) != "[]" {
		t.Fatal(w.Code, w.Body.String())
	}
	first := decodePlan(call("PUT", "/api/v1/monitor/plan", `{"enabled":true,"group":"ChatGPT","profile_id":"chatgpt","nodes":["node"]}`, true), 200)
	if w := call("GET", "/api/v1/monitor", "", true); w.Code != 200 {
		t.Fatal("single-task compatibility", w.Code)
	}
	for _, body := range []string{
		`{"group":"Video","profile_id":"chatgpt","nodes":["missing"],"enabled":false}`,
		`{"group":"Video","profile_id":"chatgpt","nodes":["node"],"revision":1,"enabled":false}`,
		`{"group":"ChatGPT","profile_id":"chatgpt","nodes":["node"],"enabled":false}`,
	} {
		if w := call("POST", "/api/v1/monitor/tasks", body, true); w.Code != 409 {
			t.Fatal("invalid task accepted", w.Code, w.Body.String())
		}
	}
	second := decodePlan(call("POST", "/api/v1/monitor/tasks", `{"enabled":false,"group":"Video","profile_id":"chatgpt","candidate_limit":12,"nodes":["node"],"auto_switch":true}`, true), 201)
	var snapshot struct {
		Tasks     []model.MonitorTask     `json:"tasks"`
		Scheduler monitor.SchedulerStatus `json:"scheduler"`
	}
	combined := call("GET", "/api/v1/monitor/tasks?include=scheduler", "", true)
	if combined.Code != 200 || json.Unmarshal(combined.Body.Bytes(), &snapshot) != nil || len(snapshot.Tasks) != 2 || snapshot.Scheduler.Workers != monitor.BackgroundWorkers {
		t.Fatal("combined snapshot", combined.Code, combined.Body.String())
	}
	if second.TaskID == first.TaskID || second.Nodes[0].SeriesID == first.Nodes[0].SeriesID {
		t.Fatal("identities merged")
	}
	for _, endpoint := range []struct{ method, path, body string }{
		{"GET", "/monitor", ""}, {"GET", "/monitor/series", ""}, {"GET", "/monitor/revisions", ""},
		{"GET", "/monitor/incidents", ""}, {"GET", "/monitor/correlations", ""},
		{"PUT", "/monitor/plan", `{"revision":1,"enabled":false}`},
		{"PUT", "/monitor/failover", `{"revision":1,"enabled":true}`},
		{"POST", "/monitor/retest", `{"revision":1,"node_id":"node"}`},
		{"POST", "/monitor/diagnostics", `{}`},
	} {
		w := call(endpoint.method, "/api/v1"+endpoint.path, endpoint.body, true)
		if w.Code != 409 || !strings.Contains(w.Body.String(), "task_id") {
			t.Fatal(endpoint.path, w.Code, w.Body.String())
		}
	}
	for _, endpoint := range []string{"retention", "storage", "catalog?group=Video&profile_id=chatgpt"} {
		if w := call("GET", "/api/v1/monitor/"+endpoint, "", true); w.Code != 200 {
			t.Fatal("controller-wide endpoint", endpoint, w.Code, w.Body.String())
		}
	}
	path := "/api/v1/monitor/tasks/" + second.TaskID
	for _, enabled := range []string{"", `,"enabled":null`, `,"enabled":"false"`} {
		if w := call("PUT", path, `{"revision":1`+enabled+`}`, true); w.Code != 400 {
			t.Fatal("missing/invalid enabled accepted", w.Code)
		}
	}
	updated := decodePlan(call("PUT", path, `{"revision":1,"enabled":false,"candidate_limit":20}`, true), 200)
	if updated.TaskID != second.TaskID || updated.ID != second.ID || updated.CandidateLimit != 20 || !updated.AutoSwitch {
		t.Fatal(updated)
	}
	if w := call("PUT", path, `{"revision":1,"enabled":false}`, true); w.Code != 409 {
		t.Fatal("stale update", w.Code)
	}
	var revisions []model.MonitorRevision
	w := call("GET", path+"/revisions", "", true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &revisions) != nil || len(revisions) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, revision := range revisions {
		if revision.Plan.TaskID != second.TaskID {
			t.Fatal("revision leakage", revision)
		}
	}
	var tasks []model.MonitorTask
	w = call("GET", "/api/v1/monitor/tasks", "", true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &tasks) != nil || len(tasks) != 2 {
		t.Fatal(w.Code, w.Body.String())
	}
	for _, task := range tasks {
		if task.Plan.TaskID == first.TaskID && (!task.Scheduled || !task.Plan.Enabled || task.Plan.Revision != 1) {
			t.Fatal("original runtime mutated", task)
		}
		if task.Plan.TaskID == second.TaskID && (!task.Scheduled || task.Plan.Enabled) {
			t.Fatal("new task scheduling state", task)
		}
	}
	decodePlan(call("PUT", "/api/v1/monitor/tasks/"+first.TaskID, `{"revision":1,"enabled":false}`, true), 200)
	decodePlan(call("PUT", path, `{"revision":2,"enabled":true}`, true), 200)
	for _, p := range []model.MonitorPlan{first, second} {
		sample := model.MonitorSample{NodeID: p.Nodes[0].ID, Kind: "baseline", Slot: 0, At: time.Now().UTC(), Outcome: "success", DelayMS: 100}
		if _, e := store.RecordMonitor(context.Background(), p.ID, sample, model.MonitorState{Status: "healthy"}, nil); e != nil {
			t.Fatal(e)
		}
		base := "/api/v1/monitor/tasks/" + p.TaskID
		var overview model.MonitorOverview
		w = call("GET", base+"/overview?window=24h", "", true)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &overview) != nil || overview.Plan.TaskID != p.TaskID || len(overview.Rows) != 1 {
			t.Fatal("task overview", w.Code, w.Body.String())
		}
		var series []model.MonitorSeries
		w = call("GET", base+"/series", "", true)
		if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &series) != nil || len(series) != 1 || series[0].ID != p.Nodes[0].SeriesID {
			t.Fatal("task series", w.Code, w.Body.String())
		}
		for _, endpoint := range []string{"incidents", "correlations", "nodes/" + p.Nodes[0].SeriesID + "/timeline"} {
			w = call("GET", base+"/"+endpoint, "", true)
			if w.Code != 200 {
				t.Fatal(endpoint, w.Code, w.Body.String())
			}
		}
	}
	w = call("GET", path+"/nodes/"+first.Nodes[0].SeriesID+"/timeline", "", true)
	if w.Code != 400 {
		t.Fatal("other task's series readable", w.Code, w.Body.String())
	}
	var scheduler monitor.SchedulerStatus
	w = call("GET", "/api/v1/monitor/scheduler", "", true)
	if w.Code != 200 || json.Unmarshal(w.Body.Bytes(), &scheduler) != nil || scheduler.Workers != 2 || scheduler.MaxRequests != 360 {
		t.Fatal(w.Code, w.Body.String())
	}
	w = call("POST", path+"/retest", `{"revision":3,"node_id":"`+second.Nodes[0].ID+`"}`, true)
	if w.Code != 202 {
		t.Fatal("task retest", w.Code, w.Body.String())
	}
	decodePlan(call("PUT", path+"/failover", `{"revision":3,"enabled":false}`, true), 200)
	w = call("POST", path+"/diagnostics", `{"include_names":false,"include_legacy":false}`, true)
	if w.Code != 200 || !strings.HasPrefix(w.Body.String(), "PK") {
		t.Fatal("scoped diagnostic download", w.Code, w.Body.String())
	}
	w = call("POST", path+"/diagnostics", `{"include_legacy":true}`, true)
	if w.Code != 400 {
		t.Fatal("unattributed legacy data exported as a task", w.Code)
	}
	for _, method := range []string{"GET", "PUT"} {
		if w := call(method, "/api/v1/monitor/tasks/absent", `{"revision":1,"enabled":false}`, true); w.Code != 404 {
			t.Fatal("missing task", method, w.Code)
		}
	}
	if w := call("DELETE", path, "", true); w.Code != 405 {
		t.Fatal(w.Code)
	}
}
