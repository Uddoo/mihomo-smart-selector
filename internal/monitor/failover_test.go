package monitor

import (
	"context"
	"errors"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestFailoverElectionUsesSuccessRateThenP95AndFreshHealth(t *testing.T) {
	now := time.Now().UTC()
	score := 99.0
	o := model.MonitorOverview{Plan: &model.MonitorPlan{Enabled: true, AutoSwitch: true}, Now: now, ObservedAt: now, Current: "A"}
	row := func(name string, rate float64, p95 int) model.MonitorRow {
		return model.MonitorRow{MonitorNode: model.MonitorNode{Name: name}, State: model.MonitorState{Status: "healthy", LastAt: now, LastSuccess: now}, Metrics: model.MonitorMetrics{Samples: 8, SuccessRate: rate, P95MS: p95}}
	}
	a := row("A", .375, 3000)
	a.State.Status = "unavailable"
	a.State.Failures = 17
	b := row("B", .875, 2163)
	c := row("C", .875, 1199)
	d := row("D", .5, 100)
	d.Metrics.Score = &score
	stale := row("stale", 1, 1)
	stale.State.LastSuccess = now.Add(-5 * time.Minute)
	bad := row("bad", 1, 1)
	bad.State.Status = "unavailable"
	o.Rows = []model.MonitorRow{a, b, c, d, stale, bad}
	got := failoverCandidates(o)
	if len(got) != 3 || got[0].Name != "C" || got[1].Name != "B" {
		t.Fatalf("wrong success-rate election: %+v", got)
	}
	for _, scenario := range []string{"disabled", "paused", "current-healthy", "controller", "stale-current", "unmonitored-current"} {
		t.Run(scenario, func(t *testing.T) {
			copy := o
			p := *o.Plan
			copy.Plan = &p
			copy.Rows = append([]model.MonitorRow(nil), o.Rows...)
			switch scenario {
			case "disabled":
				p.AutoSwitch = false
			case "paused":
				p.Enabled = false
			case "current-healthy":
				copy.Rows[0].State.Status = "healthy"
			case "controller":
				copy.Issue = "offline"
			case "stale-current":
				copy.Rows[0].State.LastAt = now.Add(-2 * time.Minute)
			case "unmonitored-current":
				copy.Current = "other"
			}
			if len(failoverCandidates(copy)) != 0 {
				t.Fatal("ineligible trigger elected node")
			}
		})
	}
}

type fakeSwitchSource struct {
	*fakeSource
	switches []string
	failNode string
}

func (f *fakeSwitchSource) MonitorDelay(ctx context.Context, n model.MonitorNode, p config.Probe) (int, error) {
	if n.Name == f.failNode {
		return 0, errors.New("HTTP 504")
	}
	return f.fakeSource.MonitorDelay(ctx, n, p)
}
func (f *fakeSwitchSource) MonitorSwitch(_ context.Context, p model.MonitorPlan, previous string, n model.MonitorNode, key string) (model.SwitchEvent, error) {
	f.switches = append(f.switches, n.Name)
	f.current = n.Name
	return model.SwitchEvent{Status: "confirmed", AuditPersisted: true, Previous: previous, Selected: n.Name}, nil
}

func failoverFixture(t *testing.T) (*Manager, *fakeSwitchSource, *time.Time) {
	t.Helper()
	m, store, f, now := setup(t)
	f.nodes = append(f.nodes, model.MonitorNode{ID: "node-b", Name: "B"}, model.MonitorNode{ID: "node-c", Name: "C"})
	source := &fakeSwitchSource{fakeSource: f}
	m.source = source
	on := true
	p, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: true, AutoSwitch: &on, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"A", "B", "C"}})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(10 * time.Minute)
	for i, n := range p.Nodes {
		state := model.MonitorState{Status: "healthy", LastAt: *now, LastSuccess: *now}
		if i == 0 {
			state.Status = "unavailable"
			state.Failures = 3
		}
		for k := 0; k < 4; k++ {
			outcome := "success"
			if i == 0 || (i == 2 && k == 0) {
				outcome = "failure"
			}
			sample := model.MonitorSample{NodeID: n.ID, Kind: "baseline", Slot: int64(k), At: time.Unix(nodeAnchor(*p, i)+int64(k*Interval), 0), Outcome: outcome, DelayMS: 100}
			if _, err = store.RecordMonitor(context.Background(), p.ID, sample, state, nil); err != nil {
				t.Fatal(err)
			}
		}
		m.states[n.ID] = state
	}
	m.refresh(context.Background(), *p, *now)
	return m, source, now
}

func TestFailoverVerifiesCandidateAndFallsBackWithoutRewritingScore(t *testing.T) {
	m, source, now := failoverFixture(t)
	source.failNode = "B"
	m.failover(context.Background(), *now)
	if len(source.switches) != 1 || source.switches[0] != "C" {
		t.Fatal(source.switches)
	}
	if m.states["node-b"].Status != "suspect" {
		t.Fatal("failed verification was ignored")
	}
	o, err := m.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	for _, r := range o.Rows {
		if r.Name == "B" && (r.Metrics.Samples != 4 || r.Metrics.SuccessRate != 1) {
			t.Fatal("verification rewrote baseline", r.Metrics)
		}
	}
	m.failover(context.Background(), now.Add(time.Second))
	if len(source.switches) != 1 {
		t.Fatal("repeated failover")
	}
}

func TestAutoSwitchPreferencePersistsWithoutResettingEvidence(t *testing.T) {
	m, source, _ := failoverFixture(t)
	id := m.plan.ID
	p, err := m.SetAutoSwitch(context.Background(), m.plan.Revision, false)
	if err != nil {
		t.Fatal(err)
	}
	restarted, err := New(m.store, source)
	if err != nil {
		t.Fatal(err)
	}
	if restarted.plan.AutoSwitch || restarted.plan.ID != id || p.ID != id || len(restarted.states) != 3 {
		t.Fatal("toggle lost evidence or was not persistent")
	}
}

func TestDisableDuringFailoverVerificationPreventsSwitch(t *testing.T) {
	m, source, now := failoverFixture(t)
	started := make(chan struct{})
	done := make(chan struct{})
	source.delay = func(ctx context.Context) (int, error) { close(started); <-ctx.Done(); return 0, ctx.Err() }
	go func() { m.failover(context.Background(), *now); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("verification did not start")
	}
	if _, err := m.SetAutoSwitch(context.Background(), m.plan.Revision, false); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("verification did not cancel")
	}
	if len(source.switches) != 0 {
		t.Fatal("switched after disabling")
	}
}
