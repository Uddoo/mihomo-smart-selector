package monitor

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func candidateNames(f *fakeSource, count int) []string {
	for len(f.nodes) < count {
		name := fmt.Sprintf("node-%02d", len(f.nodes))
		f.nodes = append(f.nodes, model.MonitorNode{ID: name, Name: name, Provider: "one", Protocol: "VLESS"})
	}
	names := make([]string, count)
	for i := range names {
		names[i] = f.nodes[i].Name
	}
	return names
}

func TestCandidateLimitPersistenceCompatibilityAndHistory(t *testing.T) {
	m, store, source, now := setup(t)
	ctx := context.Background()
	// Simulate JSON saved before candidate_limit existed.
	legacy := *clonePlan(m.plan)
	legacy.CandidateLimit = 0
	legacy.Revision++
	if err := store.SaveMonitorPlan(ctx, source.MonitorScope(), legacy, 1); err != nil {
		t.Fatal(err)
	}
	m, err := New(store, source)
	if err != nil {
		t.Fatal(err)
	}
	m.now = func() time.Time { return *now }
	if m.plan.CandidateLimit != 6 {
		t.Fatal("legacy default lost", m.plan)
	}
	m.step(ctx, *now)
	before := clonePlan(m.plan)
	limit := 30
	p, err := m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: true, CandidateLimit: &limit})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != before.ID || p.Nodes[0].SeriesID != before.Nodes[0].SeriesID || p.Nodes[0].Anchor != before.Nodes[0].Anchor {
		t.Fatal("limit-only edit reset evidence", p)
	}
	names := candidateNames(source, 30)
	p, err = m.Save(ctx, model.MonitorRequest{Revision: p.Revision, Enabled: true, Group: p.Group, ProfileID: p.ProfileID, Nodes: names})
	if err != nil || len(p.Nodes) != 30 || p.CandidateLimit != 30 {
		t.Fatal(p, err)
	}
	o, err := m.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, row := range o.Rows {
		if row.ID == before.Nodes[0].ID && (row.SeriesID != before.Nodes[0].SeriesID || row.Metrics.Samples != 1) {
			t.Fatal("expansion erased history", row)
		}
	}
	restarted, err := New(store, source)
	if err != nil || restarted.plan.CandidateLimit != 30 || len(restarted.plan.Nodes) != 30 {
		t.Fatal("limit not persisted", err)
	}
	source.err = fmt.Errorf("offline")
	paused, err := restarted.Save(ctx, model.MonitorRequest{Revision: p.Revision, Enabled: false})
	if err != nil || paused.CandidateLimit != 30 {
		t.Fatal("offline pause lost limit", paused, err)
	}
	source.err = nil
	resumed, err := restarted.Save(ctx, model.MonitorRequest{Revision: paused.Revision, Enabled: true})
	if err != nil || resumed.CandidateLimit != 30 || len(resumed.Nodes) != 30 || resumed.ID != p.ID {
		t.Fatal("resume lost plan", resumed, err)
	}
}

func TestCandidateLimitRejectsInvalidRequestsWithoutChangingPlan(t *testing.T) {
	for _, tt := range []struct {
		name                 string
		limit, nodes, probes int
	}{
		{"zero", 0, 1, 1}, {"negative", -1, 1, 1}, {"above maximum", 31, 1, 1},
		{"empty", 30, 0, 1}, {"above chosen limit", 6, 7, 1}, {"above hard limit", 30, 31, 1},
		{"empty profile", 30, 1, 0}, {"too many targets", 30, 1, 7},
	} {
		t.Run(tt.name, func(t *testing.T) {
			m, store, f, _ := setup(t)
			names := candidateNames(f, tt.nodes)
			probe := f.profile.Probes[0]
			f.profile.Probes = nil
			for range tt.probes {
				f.profile.Probes = append(f.profile.Probes, probe)
			}
			if _, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: names, CandidateLimit: &tt.limit}); err == nil {
				t.Fatal("invalid plan accepted")
			}
			persisted, err := store.MonitorPlan(context.Background(), f.MonitorScope())
			if err != nil || persisted.Revision != 1 || len(persisted.Nodes) != 1 || persisted.CandidateLimit != 6 {
				t.Fatal("rejection mutated plan", persisted, err)
			}
		})
	}
}

func TestThirtyCandidatesReceiveBaselinesForMultiTargetProfiles(t *testing.T) {
	for _, probes := range []int{1, 3, 6} {
		t.Run(fmt.Sprint(probes), func(t *testing.T) {
			m, _, f, now := setup(t)
			names := candidateNames(f, 30)
			probe := f.profile.Probes[0]
			for len(f.profile.Probes) < probes {
				f.profile.Probes = append(f.profile.Probes, probe)
			}
			limit := 30
			if _, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: names, CandidateLimit: &limit}); err != nil {
				t.Fatal(err)
			}
			start := *now
			for second := 0; second < 360; second++ {
				*now = start.Add(time.Duration(second) * time.Second)
				m.step(context.Background(), *now)
			}
			o, err := m.Overview(context.Background())
			if err != nil || len(o.Rows) != 30 {
				t.Fatal(err)
			}
			for _, row := range o.Rows {
				if row.Metrics.Samples != 3 || row.Metrics.Coverage != 1 || row.State.Status != "healthy" {
					t.Fatalf("candidate starved: %+v", row)
				}
			}
		})
	}
}

func TestExpandedBudgetRemainsSharedAcrossEditsAndRollingMinute(t *testing.T) {
	m, _, f, now := setup(t)
	names := candidateNames(f, 30)
	limit := 30
	if _, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: names, CandidateLimit: &limit}); err != nil {
		t.Fatal(err)
	}
	for i := 0; i < 60; i++ {
		if !m.takeBudget(*now) {
			t.Fatalf("expanded budget exhausted at %d", i)
		}
	}
	if m.takeBudget(*now) {
		t.Fatal("expanded budget is unbounded")
	}
	// Reducing the plan must not reset requests already spent by any probe kind.
	if _, err := m.Save(context.Background(), model.MonitorRequest{Revision: 2, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: names[:1]}); err != nil {
		t.Fatal(err)
	}
	if m.takeBudget(now.Add(59 * time.Second)) {
		t.Fatal("save reset rolling budget")
	}
	for i := 0; i < 12; i++ {
		if !m.takeBudget(now.Add(time.Minute)) {
			t.Fatal("expired attempts retained")
		}
	}
	if m.takeBudget(now.Add(time.Minute)) {
		t.Fatal("unused capacity inflated budget")
	}
}
