package monitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"sort"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Manager owns one scheduler and maintenance loop for a Controller. Task keeps
// per-group state; tasks never start their own background workers.
type Manager struct {
	*TaskRuntime // legacy single-task view; never used to choose a multi-task operation
	registryMu   sync.Mutex
	createMu     sync.Mutex
	tasks        map[string]*TaskRuntime
	order        []string
	cursor       int
	busy         map[string]bool
	dispatched   map[string]int64
	storeRef     *history.Store
	sourceRef    Source
	instance     string
	clock        func() time.Time
	historySlots chan struct{}
	budget       *requestLimiter
	storageFault atomic.Bool
	closing      atomic.Bool
	stopping     bool
	cancel       context.CancelFunc
	done         chan struct{}
	shutdownDone chan struct{}
}

func New(store *history.Store, source Source) (*Manager, error) {
	var boot [12]byte
	if _, err := rand.Read(boot[:]); err != nil {
		return nil, err
	}
	m := &Manager{tasks: map[string]*TaskRuntime{}, busy: map[string]bool{}, dispatched: map[string]int64{}, storeRef: store, sourceRef: source, instance: hex.EncodeToString(boot[:]), historySlots: make(chan struct{}, 1), budget: &requestLimiter{weights: map[string]int{}}}
	m.clock = func() time.Time {
		if m.TaskRuntime != nil {
			return m.TaskRuntime.now()
		}
		return time.Now().UTC()
	}
	stored, err := store.MonitorTasks(context.Background(), source.MonitorScope())
	if err != nil {
		return nil, err
	}
	for _, item := range stored {
		r, e := newTask(m, clonePlan(&item.Plan))
		if e != nil {
			return nil, e
		}
		m.tasks[item.Plan.TaskID] = r
		m.order = append(m.order, item.Plan.TaskID)
		if item.Scheduled {
			m.TaskRuntime = r
		}
	}
	if m.TaskRuntime == nil {
		m.TaskRuntime, err = newTask(m, nil)
		if err != nil {
			return nil, err
		}
	}
	m.TaskRuntime.now = func() time.Time { return time.Now().UTC() }
	return m, nil
}

func (m *Manager) Save(ctx context.Context, r model.MonitorRequest) (*model.MonitorPlan, error) {
	m.createMu.Lock()
	defer m.createMu.Unlock()
	if err := m.CheckLegacyTask(ctx); err != nil {
		return nil, err
	}
	return m.saveAndRegister(ctx, m.TaskRuntime, r, true)
}

func (m *Manager) saveAndRegister(ctx context.Context, task *TaskRuntime, r model.MonitorRequest, implicit bool) (*model.MonitorPlan, error) {
	m.registryMu.Lock()
	stopping := m.stopping
	m.registryMu.Unlock()
	if stopping {
		return nil, fmt.Errorf("服务正在停止")
	}
	p, err := task.savePlan(ctx, r, implicit)
	if err != nil {
		return nil, err
	}
	m.registryMu.Lock()
	defer m.registryMu.Unlock()
	if _, exists := m.tasks[p.TaskID]; !exists {
		m.tasks[p.TaskID] = task
		m.order = append(m.order, p.TaskID)
		sort.Strings(m.order)
	}
	return p, nil
}

func (m *Manager) ForTask(ctx context.Context, id string) (*TaskRuntime, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	m.registryMu.Lock()
	defer m.registryMu.Unlock()
	task, ok := m.tasks[id]
	if !ok {
		return nil, history.ErrMonitorTaskNotFound
	}
	return task, nil
}

// A completion can release a slot to the next group immediately. The round-
// robin cursor survives ticks; no group gets a second turn in the same second.
func (m *Manager) dispatch(ctx context.Context, now time.Time, complete chan<- string, workers *sync.WaitGroup) {
	m.registryMu.Lock()
	defer m.registryMu.Unlock()
	if m.stopping || ctx.Err() != nil {
		return
	}
	for checked := 0; checked < len(m.order) && len(m.busy) < BackgroundWorkers; checked++ {
		m.cursor %= len(m.order)
		id := m.order[m.cursor]
		m.cursor++
		if m.busy[id] {
			continue
		}
		if last, ok := m.dispatched[id]; ok && last == now.Unix() {
			continue
		}
		if !m.tasks[id].ready(now) {
			continue
		}
		m.busy[id], m.dispatched[id] = true, now.Unix()
		task := m.tasks[id]
		workers.Add(1)
		go func() {
			defer workers.Done()
			// Slow candidates and failover verification cannot monopolize a
			// worker indefinitely. Missed slots stay missing, never backfilled.
			turn, cancel := context.WithTimeout(ctx, 45*time.Second)
			defer cancel()
			task.step(turn, m.clock())
			task.failover(turn, m.clock())
			task.correlate(turn, m.clock())
			complete <- id
		}()
	}
}

func (m *Manager) Start() {
	m.registryMu.Lock()
	defer m.registryMu.Unlock()
	if m.cancel != nil || m.stopping {
		return
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.cancel, m.done = cancel, make(chan struct{})
	go func() {
		defer close(m.done)
		complete := make(chan string, BackgroundWorkers)
		var workers, maintenance sync.WaitGroup
		maintenance.Add(1)
		go func() { defer maintenance.Done(); m.maintenanceLoop(ctx) }()
		ticker := time.NewTicker(time.Second)
		defer ticker.Stop()
		for {
			select {
			case <-ctx.Done():
				workers.Wait()
				maintenance.Wait()
				return
			case <-ticker.C:
			case id := <-complete:
				m.registryMu.Lock()
				delete(m.busy, id)
				m.registryMu.Unlock()
			}
			m.dispatch(ctx, m.clock(), complete, &workers)
		}
	}()
}

func (m *Manager) Shutdown(ctx context.Context) error {
	m.closing.Store(true)
	m.registryMu.Lock()
	if m.shutdownDone != nil {
		done := m.shutdownDone
		m.registryMu.Unlock()
		select {
		case <-done:
			return nil
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	m.shutdownDone = make(chan struct{})
	closed := m.shutdownDone
	m.stopping = true
	if m.cancel != nil {
		m.cancel()
	}
	done := m.done
	tasks := []*TaskRuntime{m.TaskRuntime}
	for _, task := range m.tasks {
		if task != m.TaskRuntime {
			tasks = append(tasks, task)
		}
	}
	m.registryMu.Unlock()
	go func() {
		defer close(closed)
		for _, task := range tasks {
			task.mu.Lock()
			task.closed = true
			if task.inflight != nil {
				task.inflight()
			}
			task.mu.Unlock()
		}
		if done != nil {
			<-done
		}
	}()
	select {
	case <-closed:
		return nil
	case <-ctx.Done():
		return ctx.Err()
	}
}

type SchedulerStatus struct {
	Workers     int  `json:"workers"`
	Running     int  `json:"running_workers"`
	MaxRequests int  `json:"max_requests_per_minute"`
	Requests    int  `json:"requests_per_minute"`
	Used        int  `json:"requests_used"`
	Suspended   bool `json:"suspended"`
}

func (m *Manager) SchedulerStatus() SchedulerStatus {
	limit, used := m.budget.snapshot(m.clock())
	m.registryMu.Lock()
	running := len(m.busy)
	if m.stopping {
		running = 0
	}
	m.registryMu.Unlock()
	return SchedulerStatus{Workers: BackgroundWorkers, Running: running, MaxRequests: MaxRequestsPerMinute, Requests: limit, Used: used, Suspended: m.storageFault.Load()}
}
