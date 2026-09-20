package monitor

import (
	"context"
	"fmt"
	"path/filepath"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type parallelSource struct {
	mu            sync.Mutex
	current       map[string]string
	seen          map[string]int
	switches      map[string]string
	block         string
	started       chan struct{}
	once          sync.Once
	active, peak  int
	metadataBlock string
	missing       string
}

func (*parallelSource) MonitorScope() string                  { return "parallel-controller" }
func (*parallelSource) MonitorReachable(context.Context) bool { return true }
func (s *parallelSource) MonitorCatalog(ctx context.Context, group, profile string) (config.ProbeProfile, []model.MonitorNode, string, error) {
	s.mu.Lock()
	block, missing, current := s.metadataBlock == group, s.missing == group, s.current[group]
	s.mu.Unlock()
	if block {
		s.once.Do(func() { close(s.started) })
		<-ctx.Done()
		return config.ProbeProfile{}, nil, "", ctx.Err()
	}
	if missing {
		return config.ProbeProfile{}, nil, "", fmt.Errorf("group removed")
	}
	if current == "" {
		current = group + "-a"
	}
	return config.ProbeProfile{ID: profile, Probes: []config.Probe{{URL: "https://example.com/"}}}, []model.MonitorNode{{ID: group + "-a", Name: group + "-a"}, {ID: group + "-b", Name: group + "-b"}}, current, nil
}
func (s *parallelSource) MonitorDelay(ctx context.Context, node model.MonitorNode, _ config.Probe) (int, error) {
	s.mu.Lock()
	s.active++
	s.peak = max(s.peak, s.active)
	s.seen[node.Name]++
	block := node.Name == s.block
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	if block {
		s.once.Do(func() { close(s.started) })
		<-ctx.Done()
		return 0, ctx.Err()
	}
	return 100, nil
}
func (s *parallelSource) MonitorSwitch(_ context.Context, p model.MonitorPlan, previous string, node model.MonitorNode, _ string) (model.SwitchEvent, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.switches[p.Group] = node.Name
	s.current[p.Group] = node.Name
	return model.SwitchEvent{Status: "confirmed", AuditPersisted: true, Group: p.Group, Previous: previous, Selected: node.Name}, nil
}

func parallelFixture(t *testing.T, groups ...string) (*Manager, *history.Store, *parallelSource, map[string]*model.MonitorPlan, *atomic.Int64) {
	t.Helper()
	source := &parallelSource{current: map[string]string{}, seen: map[string]int{}, switches: map[string]string{}, started: make(chan struct{})}
	store, err := history.Open(filepath.Join(t.TempDir(), "parallel.db"))
	if err != nil {
		t.Fatal(err)
	}
	m, err := New(store, source)
	if err != nil {
		t.Fatal(err)
	}
	clock := &atomic.Int64{}
	clock.Store(time.Now().UTC().Truncate(time.Second).Unix())
	m.now = func() time.Time { return time.Unix(clock.Load(), 0).UTC() }
	plans := map[string]*model.MonitorPlan{}
	for _, group := range groups {
		p, e := m.SaveTask(context.Background(), "", model.MonitorRequest{Enabled: true, Group: group, ProfileID: "chatgpt", Nodes: []string{group + "-a", group + "-b"}})
		if e != nil {
			t.Fatal(e)
		}
		plans[group] = p
	}
	t.Cleanup(func() {
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if e := m.Shutdown(ctx); e != nil {
			t.Error(e)
		}
		store.Close()
	})
	return m, store, source, plans, clock
}

func awaitCondition(t *testing.T, check func() bool) {
	t.Helper()
	deadline := time.NewTimer(4 * time.Second)
	defer deadline.Stop()
	tick := time.NewTicker(5 * time.Millisecond)
	defer tick.Stop()
	for {
		if check() {
			return
		}
		select {
		case <-deadline.C:
			t.Fatal("condition not reached")
		case <-tick.C:
		}
	}
}

func TestSharedSchedulerSlowGroupPauseAndShutdown(t *testing.T) {
	m, store, source, plans, clock := parallelFixture(t, "A", "B", "C", "D")
	source.block = "A-a"
	m.order = []string{plans["A"].TaskID, plans["B"].TaskID, plans["C"].TaskID, plans["D"].TaskID}
	m.Start()
	awaitCondition(t, func() bool {
		source.mu.Lock()
		defer source.mu.Unlock()
		return source.seen["A-a"] > 0 && source.seen["B-a"] > 0 && source.seen["C-a"] > 0 && source.seen["D-a"] > 0
	})
	source.mu.Lock()
	peak := source.peak
	source.mu.Unlock()
	if peak != BackgroundWorkers {
		t.Fatal("workers are unbounded or a slow group blocked the others", peak)
	}
	if _, err := m.SaveTask(context.Background(), plans["A"].TaskID, model.MonitorRequest{Revision: 1, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	awaitCondition(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.active == 0 })
	a, err := store.MonitorSamples(context.Background(), plans["A"].ID, m.now().Add(-time.Minute))
	if err != nil || len(a) != 0 {
		t.Fatal("cancelled sample was published", a, err)
	}
	clock.Add(120)
	awaitCondition(t, func() bool {
		source.mu.Lock()
		defer source.mu.Unlock()
		return source.seen["B-a"] >= 2 && source.seen["C-a"] >= 2 && source.seen["D-a"] >= 2
	})
	source.mu.Lock()
	callsA := source.seen["A-a"]
	source.mu.Unlock()
	if callsA != 1 {
		t.Fatal("paused task continued", callsA)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err = m.Shutdown(ctx); err != nil {
		t.Fatal(err)
	}
	if m.storageFault.Load() {
		t.Fatal("cancellation treated as storage failure")
	}
	restarted, err := New(store, source)
	if err != nil {
		t.Fatal(err)
	}
	defer restarted.Shutdown(context.Background())
	tasks, err := restarted.Tasks(context.Background())
	if err != nil || len(tasks) != 4 {
		t.Fatal(tasks, err)
	}
	for _, task := range tasks {
		if !task.Scheduled || task.Plan.Enabled != (task.Plan.Group != "A") {
			t.Fatal("restart lost task state", task)
		}
	}
}

func TestPauseCancelsOnlyItsMetadataRequest(t *testing.T) {
	m, _, source, plans, _ := parallelFixture(t, "A", "B")
	source.metadataBlock = "A"
	m.Start()
	select {
	case <-source.started:
	case <-time.After(3 * time.Second):
		t.Fatal("metadata did not start")
	}
	if _, err := m.SaveTask(context.Background(), plans["A"].TaskID, model.MonitorRequest{Revision: 1, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	awaitCondition(t, func() bool { source.mu.Lock(); defer source.mu.Unlock(); return source.seen["B-a"] > 0 })
	if m.storageFault.Load() {
		t.Fatal("metadata cancellation became a global fault")
	}
}

func TestSharedRollingBudgetFairSharesAndEdits(t *testing.T) {
	b := &requestLimiter{weights: map[string]int{}}
	for _, id := range []string{"a", "b", "c", "d"} {
		b.set(id, 360, true)
	}
	now := time.Now()
	var wg sync.WaitGroup
	counts := [4]atomic.Int64{}
	for i, id := range []string{"a", "b", "c", "d"} {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for n := 0; n < 300; n++ {
				if b.take(id, now) {
					counts[i].Add(1)
				}
			}
		}()
	}
	wg.Wait()
	for i := range counts {
		if counts[i].Load() != 90 {
			t.Fatal("task monopolized shared budget", counts[i].Load())
		}
	}
	b.set("a", 12, false)
	if b.take("b", now.Add(59*time.Second)) {
		t.Fatal("pause erased spent global budget")
	}
	b.set("a", 12, true)
	if b.take("a", now.Add(59*time.Second)) {
		t.Fatal("resume erased spent task budget")
	}
	if !b.take("a", now.Add(time.Minute)) {
		t.Fatal("rolling window did not expire")
	}
	limit, used := b.snapshot(now.Add(time.Minute))
	if limit != 360 || used != 1 {
		t.Fatal(limit, used)
	}
}

func seedFailedCurrent(t *testing.T, m *Manager, store *history.Store, plan *model.MonitorPlan, now time.Time) *TaskRuntime {
	t.Helper()
	r, err := m.ForTask(context.Background(), plan.TaskID)
	if err != nil {
		t.Fatal(err)
	}
	for i, node := range plan.Nodes {
		state := model.MonitorState{Status: "healthy", LastAt: now, LastSuccess: now}
		outcome := "success"
		if i == 0 {
			state.Status = "unavailable"
			state.Failures = 2
			outcome = "failure"
		}
		for slot := int64(0); slot < 3; slot++ {
			if _, err = store.RecordMonitor(context.Background(), plan.ID, model.MonitorSample{NodeID: node.ID, Kind: "baseline", Slot: slot, At: time.Unix(node.Anchor+slot*120, 0), Outcome: outcome, DelayMS: 100}, state, nil); err != nil {
				t.Fatal(err)
			}
		}
		r.states[node.ID] = state
	}
	r.refresh(context.Background(), *plan, now)
	return r
}

func TestTwoGroupFailoverPauseIsIndependent(t *testing.T) {
	m, store, source, plans, clock := parallelFixture(t, "A", "B")
	on := true
	for _, group := range []string{"A", "B"} {
		p, err := m.SaveTask(context.Background(), plans[group].TaskID, model.MonitorRequest{Revision: 1, Enabled: true, AutoSwitch: &on})
		if err != nil {
			t.Fatal(err)
		}
		plans[group] = p
	}
	clock.Add(600)
	a := seedFailedCurrent(t, m, store, plans["A"], m.now())
	b := seedFailedCurrent(t, m, store, plans["B"], m.now())
	source.block = "A-b"
	done := make(chan struct{})
	go func() { defer close(done); a.failover(context.Background(), m.now()) }()
	select {
	case <-source.started:
	case <-time.After(time.Second):
		t.Fatal("verification did not start")
	}
	b.failover(context.Background(), m.now())
	if _, err := m.SaveTask(context.Background(), plans["A"].TaskID, model.MonitorRequest{Revision: 2, Enabled: false}); err != nil {
		t.Fatal(err)
	}
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("pause did not cancel verification")
	}
	source.mu.Lock()
	defer source.mu.Unlock()
	if len(source.switches) != 1 || source.switches["B"] != "B-b" {
		t.Fatal("cross-task switch", source.switches)
	}
}

func TestMissingGroupAndSharedStorageFault(t *testing.T) {
	m, _, source, plans, _ := parallelFixture(t, "A", "B")
	source.missing = "A"
	a, _ := m.ForTask(context.Background(), plans["A"].TaskID)
	b, _ := m.ForTask(context.Background(), plans["B"].TaskID)
	a.step(context.Background(), m.now())
	b.step(context.Background(), m.now())
	if a.issue == "" || b.issue != "" || b.states["B-a"].Status != "healthy" {
		t.Fatal("group fault crossed tasks")
	}
	before, err := b.WindowOverview(context.Background(), "24h")
	if err != nil || before.Suspended {
		t.Fatal(before, err)
	}
	a.storageFailure()
	after, err := b.WindowOverview(context.Background(), "24h")
	if err != nil || !after.Suspended {
		t.Fatal("shared fault hidden by cached view", after, err)
	}
	if b.takeBudget(m.now()) {
		t.Fatal("shared storage fault allowed probing")
	}
}

func TestTaskListSummaryDoesNotWaitForBusyTask(t *testing.T) {
	m, _, _, plans, _ := parallelFixture(t, "A", "B")
	a, _ := m.ForTask(context.Background(), plans["A"].TaskID)
	a.mu.Lock()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	done := make(chan []model.MonitorTask, 1)
	go func() { items, _ := m.Tasks(ctx); done <- items }()
	select {
	case tasks := <-done:
		a.mu.Unlock()
		if len(tasks) != 2 {
			t.Fatal(tasks)
		}
		for _, task := range tasks {
			if task.Plan.TaskID == plans["A"].TaskID && task.Runtime != nil {
				t.Fatal("busy task published an inconsistent summary")
			}
			if task.Plan.TaskID == plans["B"].TaskID && task.Runtime == nil {
				t.Fatal("busy A blocked B summary")
			}
		}
	case <-ctx.Done():
		a.mu.Unlock()
		t.Fatal("task list blocked on in-flight switch")
	}
}
