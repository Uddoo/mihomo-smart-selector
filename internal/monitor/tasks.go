package monitor

import (
	"context"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) CheckLegacyTask(ctx context.Context) error {
	_, err := m.store.MonitorPlan(ctx, m.source.MonitorScope())
	return err
}

func (m *Manager) Tasks(ctx context.Context) ([]model.MonitorTask, error) {
	tasks, err := m.store.MonitorTasks(ctx, m.source.MonitorScope())
	if err != nil {
		return nil, err
	}
	m.registryMu.Lock()
	runtimes := make(map[string]*TaskRuntime, len(m.tasks))
	for id, runtime := range m.tasks {
		runtimes[id] = runtime
	}
	for i := range tasks {
		_, tasks[i].Scheduled = m.tasks[tasks[i].Plan.TaskID]
		tasks[i].Scheduled = tasks[i].Scheduled && !m.stopping
	}
	m.registryMu.Unlock()
	for i := range tasks {
		if r := runtimes[tasks[i].Plan.TaskID]; r != nil {
			tasks[i].Runtime = r.summary(tasks[i].Plan.Revision)
		}
	}
	return tasks, nil
}

// The task list must not wait behind an audited Controller switch or read all
// historical series. Omitted runtime data explicitly means "updating" in UI.
func (m *TaskRuntime) summary(revision int) *model.MonitorTaskSummary {
	if !m.mu.TryLock() {
		return nil
	}
	defer m.mu.Unlock()
	if m.plan == nil || m.plan.Revision != revision {
		return nil
	}
	out := &model.MonitorTaskSummary{Current: m.current, Issue: m.issue, Suspended: m.fault || m.owner.storageFault.Load(), ObservedAt: m.observedAt}
	for _, node := range m.plan.Nodes {
		state := m.states[node.ID]
		if !m.plan.Enabled || out.Suspended || out.Issue != "" || !fresh(state.LastAt, m.now(), 240*time.Second) {
			out.Unknown++
			continue
		}
		if state.Status == "suspect" || state.Status == "unavailable" || state.Status == "recovering" {
			out.Unhealthy++
		}
		if state.Status == "unknown" || state.Status == "" {
			out.Unknown++
		}
	}
	return out
}

func (m *Manager) Task(ctx context.Context, taskID string) (model.MonitorTask, error) {
	task, err := m.store.MonitorTask(ctx, m.source.MonitorScope(), taskID)
	if err != nil {
		return task, err
	}
	m.registryMu.Lock()
	defer m.registryMu.Unlock()
	_, task.Scheduled = m.tasks[taskID]
	task.Scheduled = task.Scheduled && !m.stopping
	return task, nil
}

// Empty taskID creates a task; updates always address a stable task identity.
// Every saved task joins the shared scheduler; Enabled controls its sampling.
func (m *Manager) SaveTask(ctx context.Context, taskID string, r model.MonitorRequest) (*model.MonitorPlan, error) {
	if taskID != "" {
		task, err := m.ForTask(ctx, taskID)
		if err != nil {
			return nil, err
		}
		return m.saveAndRegister(ctx, task, r, false)
	}
	m.createMu.Lock()
	defer m.createMu.Unlock()
	m.registryMu.Lock()
	empty := len(m.tasks) == 0
	m.registryMu.Unlock()
	task := m.TaskRuntime
	if !empty {
		var err error
		task, err = newTask(m, nil)
		if err != nil {
			return nil, err
		}
	}
	return m.saveAndRegister(ctx, task, r, false)
}

func (m *Manager) TaskRevisions(ctx context.Context, taskID string) ([]model.MonitorRevision, error) {
	return m.store.MonitorTaskRevisions(ctx, m.source.MonitorScope(), taskID)
}

func (m *TaskRuntime) taskID() string {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.plan == nil {
		return ""
	}
	return m.plan.TaskID
}
