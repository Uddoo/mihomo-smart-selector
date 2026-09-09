package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestCandidateEditsPreserveIndependentHistory(t *testing.T) {
	m, store, f, now := setup(t)
	ctx := context.Background()
	m.step(ctx, *now)
	first := m.plan.Nodes[0]
	f.nodes = append(f.nodes, model.MonitorNode{ID: "b", Name: "B", Provider: "two", Protocol: "VLESS"})
	*now = now.Add(5 * time.Minute)
	p, err := m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"B", "A"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Nodes[1].SeriesID != first.SeriesID || p.Nodes[1].Anchor != first.Anchor {
		t.Fatal("reordered node lost identity")
	}
	o, err := m.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range o.Rows {
		if r.Name == "A" && r.Metrics.Samples != 1 {
			t.Fatal("adding node reset history")
		}
	}
	_, err = m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"B"}})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(5 * time.Minute)
	p, err = m.Save(ctx, model.MonitorRequest{Revision: m.plan.Revision, Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"A", "B"}})
	if err != nil {
		t.Fatal(err)
	}
	if p.Nodes[0].SeriesID != first.SeriesID {
		t.Fatal("re-adding node reset series")
	}
	revisions, err := store.MonitorRevisions(ctx, f.MonitorScope())
	if err != nil || len(revisions) != 4 {
		t.Fatal(revisions, err)
	}
	o, err = m.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range o.Rows {
		if r.Name == "A" && (r.Metrics.Expected != 6 || r.Metrics.Samples != 1) {
			t.Fatal("unobserved gap hidden", r.Metrics)
		}
	}
}
