package monitor

import (
	"context"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type egressFixture struct {
	*fakeSource
	clock       func() time.Time
	egressCalls int
	after       func()
}

func (f *egressFixture) MonitorEgress(ctx context.Context, group string, node model.MonitorNode, reserve func() bool) model.MonitorEgress {
	f.egressCalls++
	if !reserve() {
		return model.MonitorEgress{Status: "budget"}
	}
	if f.after != nil {
		f.after()
	}
	return model.MonitorEgress{Status: "ok", IP: "8.8.8.8", CountryCode: "US", ObservedAt: f.clock()}
}

func TestEgressSchedulerFrequencySnapshotAndManualRetest(t *testing.T) {
	m, _, base, now := setup(t)
	f := &egressFixture{fakeSource: base, clock: func() time.Time { return *now }}
	m.source = f
	m.step(context.Background(), *now)
	o, err := m.Overview(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	if f.egressCalls != 1 || o.Rows[0].Egress == nil || o.Rows[0].Egress.IP != "8.8.8.8" || o.Rows[0].Metrics.Samples != 1 {
		t.Fatal("egress missing or health changed", o)
	}
	o.Rows[0].Egress.IP = "corrupted"
	for range 3 {
		o, err = m.Overview(context.Background())
		if err != nil || o.Rows[0].Egress.IP != "8.8.8.8" {
			t.Fatal("snapshot alias", err)
		}
	}
	if f.egressCalls != 1 {
		t.Fatal("GET triggered a probe")
	}
	*now = now.Add(120 * time.Second)
	m.step(context.Background(), *now)
	if f.egressCalls != 1 {
		t.Fatal("queried before interval")
	}
	*now = now.Add(480 * time.Second)
	m.step(context.Background(), *now)
	if f.egressCalls != 2 {
		t.Fatal("did not refresh after interval")
	}
	if err = m.Retest("node-a", 1); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Second)
	m.step(context.Background(), *now)
	if f.egressCalls != 3 {
		t.Fatal("manual retest did not refresh egress")
	}
	if _, err = m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	*now = now.Add(time.Hour)
	m.step(context.Background(), *now)
	if f.egressCalls != 3 {
		t.Fatal("paused task probed")
	}
}

func TestEgressDropsResultsFromChangedPlans(t *testing.T) {
	m, _, base, now := setup(t)
	f := &egressFixture{fakeSource: base, clock: func() time.Time { return *now }}
	m.source = f
	f.after = func() { m.mu.Lock(); m.plan.Revision++; m.mu.Unlock() }
	plan := *m.plan
	node := plan.Nodes[0]
	m.captureEgress(context.Background(), plan, node, false)
	if len(m.egress) != 0 {
		t.Fatal("late result from obsolete plan was committed")
	}
	plan.ProfileID = "claude"
	m.captureEgress(context.Background(), plan, node, true)
	if f.egressCalls != 1 {
		t.Fatal("non-ChatGPT task queried egress")
	}
}
