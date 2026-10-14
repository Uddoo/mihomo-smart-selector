package monitor

import (
	"context"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

const egressInterval = 10 * time.Minute

type egressSource interface {
	MonitorEgress(context.Context, string, model.MonitorNode, func() bool) model.MonitorEgress
}

// Runs only from the scheduler after successful sampling, never from GET routes.
func (m *TaskRuntime) captureEgress(ctx context.Context, plan model.MonitorPlan, node model.MonitorNode, manual bool) {
	source, ok := m.source.(egressSource)
	if !ok || plan.ProfileID != "chatgpt" || ctx.Err() != nil {
		return
	}
	m.mu.Lock()
	previous := m.egress[node.ID]
	now := m.now()
	interval := egressInterval
	if previous.Status != "ok" || previous.Intelligence != nil && previous.Intelligence.Status == "budget" {
		interval = time.Minute
	}
	for _, data := range previous.IntelligenceSources {
		if data.Status == "budget" {
			interval = time.Minute
			break
		}
	}
	valid := !m.closed && !m.fault && m.plan != nil && m.plan.Revision == plan.Revision && m.plan.Enabled
	m.mu.Unlock()
	if !valid || !manual && !previous.AttemptedAt.IsZero() && now.Sub(previous.AttemptedAt) < interval {
		return
	}
	value := source.MonitorEgress(ctx, plan.Group, node, func() bool { return m.takeBudget(m.now()) })
	if ctx.Err() != nil {
		return
	}
	value.AttemptedAt = m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.fault || m.plan == nil || m.plan.Revision != plan.Revision || !m.plan.Enabled {
		return
	}
	if m.egress == nil {
		m.egress = map[string]model.MonitorEgress{}
	}
	m.egress[node.ID] = value
	m.dataVersion++
}
