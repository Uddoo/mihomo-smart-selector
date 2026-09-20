package history

import (
	"context"
	"path/filepath"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestTaskHistoryEventsAndCorrelationsStayScoped(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "scope.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	plans := []model.MonitorPlan{taskPlan("a", "A", at), taskPlan("b", "B", at)}
	for i := range plans {
		p := &plans[i]
		if err = s.PrepareMonitorPlan(ctx, "scope", p); err != nil {
			t.Fatal(err)
		}
		if err = s.SaveMonitorTask(ctx, "scope", *p, 0); err != nil {
			t.Fatal(err)
		}
		sample := model.MonitorSample{NodeID: "a", Kind: "baseline", Slot: 0, At: at, Outcome: "failure"}
		if _, err = s.RecordMonitor(ctx, p.ID, sample, model.MonitorState{Status: "unavailable"}, &model.MonitorEvent{NodeID: "a", NodeName: "A", At: at, Status: "unavailable"}); err != nil {
			t.Fatal(err)
		}
		if err = s.RecordMonitorSystem(ctx, "scope", p.ID, "unknown", p.Group, at); err != nil {
			t.Fatal(err)
		}
		if _, err = s.RecordSwitch(ctx, model.SwitchEvent{ControllerScope: "scope", ScanID: "monitor:" + p.ID, Group: p.Group, Status: "confirmed", CreatedAt: at}); err != nil {
			t.Fatal(err)
		}
		o := model.MonitorCorrelation{PlanID: p.ID, Provider: "P", Monitored: 3, Comparable: 3, Failed: 3, Reliable: true, UpdatedAt: at, Signature: "1"}
		if err = s.ObserveMonitorCorrelation(ctx, "scope", o); err != nil {
			t.Fatal(err)
		}
		o.Signature = "2"
		if err = s.ObserveMonitorCorrelation(ctx, "scope", o); err != nil {
			t.Fatal(err)
		}
	}
	// Identical group names on another Controller cannot enter this task's audit.
	if _, err = s.RecordSwitch(ctx, model.SwitchEvent{ControllerScope: "other", Group: "A", Status: "confirmed", CreatedAt: at}); err != nil {
		t.Fatal(err)
	}
	if err = s.EndMonitorCorrelations(ctx, "scope", plans[0].ID, true, at.Add(time.Second), "a"); err != nil {
		t.Fatal(err)
	}
	for i, p := range plans {
		correlations, e := s.MonitorCorrelations(ctx, "scope", p.TaskID)
		if e != nil || len(correlations) != 1 {
			t.Fatal(correlations, e)
		}
		want := "active"
		if i == 0 {
			want = "scope_changed"
		}
		if correlations[0].Status != want {
			t.Fatal("another task ended this incident", correlations)
		}
		series, e := s.MonitorSeries(ctx, "scope", p.TaskID)
		if e != nil || len(series) != 1 || series[0].ID != p.Nodes[0].SeriesID {
			t.Fatal("series crossed tasks", series, e)
		}
		if _, e = s.SeriesDefinition(ctx, "scope", plans[1-i].Nodes[0].SeriesID, p.TaskID); e == nil {
			t.Fatal("cross-task series readable")
		}
		exported, e := s.DiagnosticSamples(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), 100, false, p.TaskID)
		if e != nil || len(exported.Samples) != 1 || exported.Samples[0].SeriesID != p.Nodes[0].SeriesID {
			t.Fatal(exported, e)
		}
		page, e := s.MonitorActivities(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), "", 100, p.TaskID)
		if e != nil || len(page.Items) != 5 {
			t.Fatal("history attribution", len(page.Items), page, e)
		}
		for _, event := range page.Items {
			if event.Group != "" && event.Group != p.Group {
				t.Fatal("another group's event", event)
			}
		}
	}
	page, err := s.MonitorActivities(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), "", 1, "a")
	if err != nil || page.NextCursor == "" {
		t.Fatal(page, err)
	}
	if _, err = s.MonitorActivities(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), page.NextCursor, 1, "b"); err == nil {
		t.Fatal("cross-task cursor accepted")
	}
	if _, err = s.DiagnosticSamples(ctx, "scope", at.Add(-time.Minute), at.Add(time.Minute), 100, true, "a"); err == nil {
		t.Fatal("unmapped legacy evidence attributed to a task")
	}
}

func TestReusedGroupNameDoesNotReattributeSwitchAudit(t *testing.T) {
	s, err := Open(filepath.Join(t.TempDir(), "reuse.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close()
	ctx := context.Background()
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Second)
	a := taskPlan("a", "G", at)
	if err = s.SaveMonitorTask(ctx, "scope", a, 0); err != nil {
		t.Fatal(err)
	}
	record := func(plan, selected string, seconds int) {
		t.Helper()
		if _, e := s.RecordSwitch(ctx, model.SwitchEvent{ControllerScope: "scope", ScanID: plan, Group: "G", Selected: selected, Status: "confirmed", CreatedAt: at.Add(time.Duration(seconds) * time.Second)}); e != nil {
			t.Fatal(e)
		}
	}
	record("monitor:"+a.ID, "old-auto", 10)
	record("scan-old", "old-manual", 20)
	a.ID = "a-new-epoch"
	a.Group = "H"
	a.Revision = 2
	a.UpdatedAt = at.Add(30 * time.Second)
	if err = s.SaveMonitorTask(ctx, "scope", a, 1); err != nil {
		t.Fatal(err)
	}
	b := taskPlan("b", "G", at.Add(40*time.Second))
	if err = s.SaveMonitorTask(ctx, "scope", b, 0); err != nil {
		t.Fatal(err)
	}
	record("monitor:"+b.ID, "new-auto", 50)
	record("scan-new", "new-manual", 60)
	for _, item := range []struct{ id, prefix string }{{"a", "old-"}, {"b", "new-"}} {
		page, e := s.MonitorActivities(ctx, "scope", at, at.Add(time.Minute), "", 100, item.id)
		if e != nil {
			t.Fatal(e)
		}
		switches := 0
		for _, event := range page.Items {
			if event.Kind == "automatic_switch" || event.Kind == "manual_switch" {
				switches++
				if !strings.HasPrefix(event.Selected, item.prefix) {
					t.Fatal("audit reassigned to new owner", event)
				}
			}
		}
		if switches != 2 {
			t.Fatal("lost task audit", item.id, switches, page)
		}
	}
}
