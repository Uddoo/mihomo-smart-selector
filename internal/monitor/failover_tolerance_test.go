package monitor

import (
	"context"
	"fmt"
	"math"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestFailoverToleranceRanksLatencyWithinOneFixedBand(t *testing.T) {
	now := time.Now().UTC()
	row := func(name string, rate float64, p95 int) model.MonitorRow {
		return model.MonitorRow{MonitorNode: model.MonitorNode{Name: name}, State: model.MonitorState{Status: "healthy", LastAt: now, LastSuccess: now}, Metrics: model.MonitorMetrics{Samples: 500, SuccessRate: rate, P95MS: p95}}
	}
	failed := row("current", 1, 1)
	failed.State.Status = "unavailable"
	failed.State.Failures = 2
	rows := []model.MonitorRow{failed, row("slow", .999, 1800), row("fast", .995, 220), row("boundary", .989, 350), row("fallback", .98, 50), row("fallback-faster", .97, 10)}
	for _, tt := range []struct {
		tolerance float64
		want      []string
	}{
		{0, []string{"slow", "fast", "boundary", "fallback", "fallback-faster"}},
		{1, []string{"fast", "boundary", "slow", "fallback", "fallback-faster"}},
		{100, []string{"fallback-faster", "fallback", "fast", "boundary", "slow"}},
	} {
		o := model.MonitorOverview{Plan: &model.MonitorPlan{Enabled: true, AutoSwitch: true, FailoverTolerancePP: tt.tolerance}, Now: now, ObservedAt: now, Current: "current", Rows: rows}
		got := []string{}
		for _, r := range failoverCandidates(o) {
			got = append(got, r.Name)
		}
		if !reflect.DeepEqual(got, tt.want) {
			t.Fatalf("tolerance %g: %v != %v", tt.tolerance, got, tt.want)
		}
	}
	// Equal P95 resolves by success rate then name; names/input order cannot
	// pull another node into the preferred band through chained differences.
	o := model.MonitorOverview{Plan: &model.MonitorPlan{Enabled: true, AutoSwitch: true, FailoverTolerancePP: 1}, Now: now, ObservedAt: now, Current: "current", Rows: []model.MonitorRow{failed, row("Z", .99, 300), row("A", .99, 300), row("best", 1, 300), row("chained", .98, 1)}}
	got := failoverCandidates(o)
	for i, want := range []string{"best", "A", "Z", "chained"} {
		if got[i].Name != want {
			t.Fatal(got)
		}
	}
}

func TestFailoverTolerancePersistsWithoutResettingHistory(t *testing.T) {
	m, store, source, _ := setup(t)
	ctx := context.Background()
	original := clonePlan(m.plan)
	if original.FailoverTolerancePP != model.DefaultFailoverTolerancePP {
		t.Fatal(original)
	}
	tolerance := 2.5
	p, err := m.Save(ctx, model.MonitorRequest{Revision: original.Revision, Enabled: true, FailoverTolerancePP: &tolerance})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID != original.ID || !p.CreatedAt.Equal(original.CreatedAt) || !reflect.DeepEqual(p.Nodes, original.Nodes) {
		t.Fatal("tolerance edit reset evidence", p)
	}
	for _, bad := range []float64{-1, 100.1, math.NaN(), math.Inf(1)} {
		if _, err = m.Save(ctx, model.MonitorRequest{Revision: p.Revision, Enabled: false, FailoverTolerancePP: &bad}); err == nil {
			t.Fatalf("accepted %g", bad)
		}
		if m.plan.Revision != p.Revision || m.plan.FailoverTolerancePP != 2.5 {
			t.Fatal("invalid request mutated plan")
		}
	}
	for _, enabled := range []bool{false, true} {
		p, err = m.Save(ctx, model.MonitorRequest{Revision: p.Revision, Enabled: enabled})
		if err != nil || p.FailoverTolerancePP != 2.5 {
			t.Fatal("toggle lost tolerance", p, err)
		}
	}
	zero := 0.0
	p, err = m.Save(ctx, model.MonitorRequest{Revision: p.Revision, Enabled: false, FailoverTolerancePP: &zero})
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := New(store, source)
	if err != nil || restarted.plan.FailoverTolerancePP != 0 {
		t.Fatal("explicit zero lost on restart", err)
	}
	tasks, err := restarted.Tasks(ctx)
	if err != nil || tasks[0].Plan.FailoverTolerancePP != 0 {
		t.Fatal("task read disagrees", tasks, err)
	}
}

func TestFailoverToleranceRetestsFastCandidateAndFallsBack(t *testing.T) {
	for _, failFast := range []bool{false, true} {
		t.Run(fmt.Sprint(failFast), func(t *testing.T) {
			m, store, f, now := setup(t)
			f.nodes = append(f.nodes, model.MonitorNode{ID: "slow", Name: "slow"}, model.MonitorNode{ID: "fast", Name: "fast"})
			source := &fakeSwitchSource{fakeSource: f}
			if failFast {
				source.failNode = "fast"
			}
			m.source = source
			on := true
			p, err := m.Save(context.Background(), model.MonitorRequest{Revision: m.plan.Revision, Enabled: true, AutoSwitch: &on, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"A", "slow", "fast"}})
			if err != nil {
				t.Fatal(err)
			}
			*now = now.Add(200 * time.Minute)
			for i, n := range p.Nodes {
				state := model.MonitorState{Status: "healthy", LastAt: *now, LastSuccess: *now}
				if i == 0 {
					state.Status = "unavailable"
					state.Failures = 2
				}
				for slot := 0; slot < 100; slot++ {
					outcome, delay := "success", 1800
					if i == 2 {
						delay = 200
					}
					if i == 0 || (i == 2 && slot == 0) {
						outcome = "failure"
					}
					sample := model.MonitorSample{NodeID: n.ID, Kind: "baseline", Slot: int64(slot), At: time.Unix(nodeAnchor(*p, i)+int64(slot*Interval), 0), Outcome: outcome, DelayMS: delay}
					if _, err = store.RecordMonitor(context.Background(), p.ID, sample, state, nil); err != nil {
						t.Fatal(err)
					}
				}
				m.states[n.ID] = state
			}
			m.refresh(context.Background(), *p, *now)
			m.failover(context.Background(), *now)
			want := "fast"
			if failFast {
				want = "slow"
			}
			if !reflect.DeepEqual(source.switches, []string{want}) {
				t.Fatal(source.switches)
			}
			o, err := m.Overview(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			for _, r := range o.Rows {
				if r.Name == "fast" && (r.Metrics.Samples != 100 || r.Metrics.SuccessRate != .99) {
					t.Fatal("retest altered baseline", r.Metrics)
				}
			}
		})
	}
}
