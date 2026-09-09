package monitor

import (
	"archive/zip"
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"io"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestProviderCorrelationIsScopedAndRequiresContrast(t *testing.T) {
	now := time.Now().UTC()
	p := model.MonitorPlan{ID: "p", Nodes: []model.MonitorNode{{ID: "a", Name: "A", Provider: "one"}, {ID: "b", Name: "B", Provider: "one"}, {ID: "c", Name: "C", Provider: "one"}, {ID: "d", Name: "D", Provider: "two"}}}
	states := map[string]model.MonitorState{}
	for _, n := range p.Nodes {
		states[n.ID] = model.MonitorState{Status: "unavailable", LastAt: now}
	}
	check := func(contrast bool) {
		t.Helper()
		for _, o := range correlationObservations(p, states, "", now) {
			if o.Provider == "one" && (o.Failed != 3 || o.Comparable != 3 || o.OtherProviderHealthy != contrast) {
				t.Fatal(o)
			}
		}
	}
	check(false)
	states["d"] = model.MonitorState{Status: "healthy", LastAt: now}
	check(true)
	for _, o := range correlationObservations(p, states, "controller offline", now) {
		if o.Reliable {
			t.Fatal("controller outage attributed to provider")
		}
	}
}

func TestDiagnosticZIPRedactionAndCSVInjection(t *testing.T) {
	m, store, f, now := setup(t)
	f.nodes[0].Name = "=1+1"
	f.nodes[0].Provider = "private-provider"
	p, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: true, Group: "private-group", ProfileID: "chatgpt", Nodes: []string{"=1+1"}})
	if err != nil {
		t.Fatal(err)
	}
	v := model.MonitorSample{NodeID: p.Nodes[0].ID, Kind: "baseline", Slot: 0, At: *now, Outcome: "failure", Reason: "SECRET_CANARY https://private.example/token"}
	if _, err = store.RecordMonitor(context.Background(), p.ID, v, model.MonitorState{}, &model.MonitorEvent{NodeID: v.NodeID, NodeName: "=1+1", At: *now, Status: "unavailable", Message: v.Reason}); err != nil {
		t.Fatal(err)
	}
	o := model.MonitorCorrelation{PlanID: p.ID, Provider: "private-provider", Failed: 3, Comparable: 3, Reliable: true, Nodes: []string{"=1+1", "other-private-node"}, UpdatedAt: *now, Signature: "one"}
	if err = store.ObserveMonitorCorrelation(context.Background(), f.MonitorScope(), o); err != nil {
		t.Fatal(err)
	}
	o.Signature = "two"
	if err = store.ObserveMonitorCorrelation(context.Background(), f.MonitorScope(), o); err != nil {
		t.Fatal(err)
	}
	read := func(names bool) map[string][]byte {
		t.Helper()
		data, e := m.Diagnostics(context.Background(), DiagnosticRequest{From: now.Add(-time.Hour), To: *now, IncludeNames: names})
		if e != nil {
			t.Fatal(e)
		}
		z, e := zip.NewReader(bytes.NewReader(data), int64(len(data)))
		if e != nil {
			t.Fatal(e)
		}
		out := map[string][]byte{}
		for _, file := range z.File {
			r, e := file.Open()
			if e != nil {
				t.Fatal(e)
			}
			out[file.Name], e = io.ReadAll(r)
			r.Close()
			if e != nil {
				t.Fatal(e)
			}
		}
		return out
	}
	files := read(false)
	if len(files) != 5 {
		t.Fatal("wrong bundle contents")
	}
	for name, data := range files {
		for _, secret := range []string{"SECRET_CANARY", "private.example", "private-provider", "private-group", "=1+1"} {
			if bytes.Contains(data, []byte(secret)) {
				t.Fatal("export leaked", name, secret)
			}
		}
	}
	var manifest map[string]any
	if err = json.Unmarshal(files["manifest.json"], &manifest); err != nil || manifest["sample_count"] != float64(1) {
		t.Fatal(manifest, err)
	}
	files = read(true)
	rows, err := csv.NewReader(bytes.NewReader(files["samples.csv"])).ReadAll()
	if err != nil {
		t.Fatal(err)
	}
	if rows[1][1] != "'=1+1" {
		t.Fatal("CSV formula not escaped", rows)
	}
	for _, data := range files {
		if strings.Contains(string(data), "SECRET_CANARY") {
			t.Fatal("raw errors included with real names")
		}
	}
	if _, err = m.Diagnostics(context.Background(), DiagnosticRequest{From: now.Add(-8 * 24 * time.Hour), To: *now}); err == nil {
		t.Fatal("oversized window accepted")
	}
}
