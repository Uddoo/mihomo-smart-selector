package monitor

import (
	"context"
	"errors"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

type fakeSource struct {
	profile       config.ProbeProfile
	nodes         []model.MonitorNode
	err, delayErr error
	current       string
	reachable     bool
	calls         int
	delay         func(context.Context) (int, error)
}

func (f *fakeSource) MonitorScope() string { return "fixture" }
func (f *fakeSource) MonitorCatalog(context.Context, string, string) (config.ProbeProfile, []model.MonitorNode, string, error) {
	return f.profile, f.nodes, f.current, f.err
}
func (f *fakeSource) MonitorDelay(ctx context.Context, _ model.MonitorNode, _ config.Probe) (int, error) {
	f.calls++
	if f.delay != nil {
		return f.delay(ctx)
	}
	return 150, f.delayErr
}
func (f *fakeSource) MonitorReachable(context.Context) bool { return f.reachable }

func setup(t *testing.T) (*Manager, *history.Store, *fakeSource, *time.Time) {
	t.Helper()
	store, err := history.Open(filepath.Join(t.TempDir(), "monitor.db"))
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { store.Close() })
	f := &fakeSource{profile: config.ProbeProfile{ID: "chatgpt", Probes: []config.Probe{{Name: "trace", URL: "https://example.com/health", ExpectedStatus: "200"}}}, nodes: []model.MonitorNode{{ID: "node-a", Name: "A", Provider: "one", Protocol: "VLESS"}}, current: "A", reachable: true}
	m, err := New(store, f)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC().Truncate(time.Second)
	m.now = func() time.Time { return now }
	_, err = m.Save(context.Background(), model.MonitorRequest{Enabled: true, Group: "ChatGPT", ProfileID: "chatgpt", Nodes: []string{"A"}})
	if err != nil {
		t.Fatal(err)
	}
	return m, store, f, &now
}

func TestPersistentBaselinePauseResumeAndRevision(t *testing.T) {
	m, store, f, now := setup(t)
	ctx := context.Background()
	m.step(ctx, *now)
	o, err := m.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Rows[0].Metrics.Samples != 1 {
		t.Fatal(o)
	}
	if _, err = m.Save(ctx, model.MonitorRequest{Revision: 0, Enabled: false}); err == nil {
		t.Fatal("stale tab changed plan")
	}
	paused, err := m.Save(ctx, model.MonitorRequest{Revision: 1, Enabled: false})
	if err != nil {
		t.Fatal(err)
	}
	*now = now.Add(10 * time.Minute)
	m.step(ctx, *now)
	if f.calls != 1 {
		t.Fatal("paused plan probed")
	}
	resumed, err := m.Save(ctx, model.MonitorRequest{Revision: 2, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if resumed.ID != paused.ID {
		t.Fatal("resume reset history")
	}
	m.step(ctx, *now)
	restarted, err := New(store, f)
	if err != nil {
		t.Fatal(err)
	}
	restarted.now = func() time.Time { return *now }
	calls := f.calls
	restarted.step(ctx, *now)
	if f.calls != calls {
		t.Fatal("restart replayed current slot")
	}
	o, err = restarted.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Rows[0].Metrics.Samples != 2 || o.Rows[0].Metrics.Expected != 6 || o.Rows[0].Metrics.Coverage >= .8 {
		t.Fatalf("gap fabricated evidence: %+v", o.Rows[0].Metrics)
	}
}

func TestUnavailableRecoveryAndBaselineHistory(t *testing.T) {
	m, _, f, now := setup(t)
	ctx := context.Background()
	start := *now
	f.delayErr = errors.New("controller returned HTTP 504")
	m.step(ctx, *now)
	*now = start.Add(10 * time.Second)
	m.step(ctx, *now)
	if m.states["node-a"].Status != "unavailable" {
		t.Fatal(m.states)
	}
	f.delayErr = nil
	*now = start.Add(120 * time.Second)
	m.step(ctx, *now)
	*now = start.Add(130 * time.Second)
	m.step(ctx, *now)
	*now = start.Add(240 * time.Second)
	m.step(ctx, *now)
	o, err := m.Overview(ctx)
	if err != nil {
		t.Fatal(err)
	}
	if o.Rows[0].State.Status != "healthy" || o.Rows[0].Metrics.Samples != 3 || o.Rows[0].Metrics.SuccessRate != 2.0/3 || o.Rows[0].Metrics.Incidents != 1 {
		t.Fatalf("recovery erased history: %+v", o.Rows[0])
	}
	if len(o.Events) != 4 {
		t.Fatalf("state changes not deduplicated: %+v", o.Events)
	}
}

func TestControllerFailureBusyAndProfileChangeAreUnknown(t *testing.T) {
	for _, scenario := range []string{"controller", "busy", "profile", "removed"} {
		t.Run(scenario, func(t *testing.T) {
			m, _, f, now := setup(t)
			switch scenario {
			case "controller":
				f.err = errors.New("Controller unavailable")
			case "busy":
				f.delayErr = scan.ErrMonitorBusy
			case "profile":
				f.profile.Probes[0].URL = "https://example.com/new"
			case "removed":
				f.nodes = nil
			}
			m.step(context.Background(), *now)
			o, err := m.Overview(context.Background())
			if err != nil {
				t.Fatal(err)
			}
			if o.Rows[0].State.Status != "unknown" || o.Rows[0].Metrics.Samples != 0 || o.Rows[0].Metrics.Incidents != 0 {
				t.Fatal(o.Rows[0])
			}
		})
	}
}

func TestPauseCancelsProbeAndDoesNotPublishLateResult(t *testing.T) {
	m, store, f, now := setup(t)
	started := make(chan struct{})
	done := make(chan struct{})
	f.delay = func(ctx context.Context) (int, error) { close(started); <-ctx.Done(); return 0, ctx.Err() }
	go func() { m.step(context.Background(), *now); close(done) }()
	select {
	case <-started:
	case <-time.After(time.Second):
		t.Fatal("probe did not start")
	}
	if _, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pause did not cancel")
	}
	samples, err := store.MonitorSamples(context.Background(), m.plan.ID, now.Add(-time.Hour))
	if err != nil || len(samples) != 0 {
		t.Fatalf("late sample: %+v %v", samples, err)
	}
}

func TestBudgetAndStorageFailureStopWork(t *testing.T) {
	m, store, f, now := setup(t)
	for i := 0; i < 12; i++ {
		if !m.takeBudget(*now) {
			t.Fatal("early budget limit")
		}
	}
	if m.takeBudget(*now) {
		t.Fatal("unbounded budget")
	}
	m.step(context.Background(), *now)
	if f.calls != 0 {
		t.Fatal("probed over budget")
	}
	*now = now.Add(time.Hour)
	store.Close()
	m.step(context.Background(), *now)
	if !m.fault || f.calls != 0 {
		t.Fatal("storage fault did not stop sampling")
	}
}

func TestChangingNodeIdentityStartsNewEvidence(t *testing.T) {
	m, _, f, now := setup(t)
	m.step(context.Background(), *now)
	id := m.plan.ID
	f.nodes[0].Provider = "replacement"
	f.nodes[0].ID = "new-identity"
	p, err := m.Save(context.Background(), model.MonitorRequest{Revision: 1, Enabled: true})
	if err != nil {
		t.Fatal(err)
	}
	if p.ID == id || len(m.states) != 0 {
		t.Fatal("identity change merged history")
	}
}
