package monitor

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

type switchSource interface {
	MonitorSwitch(context.Context, model.MonitorPlan, string, model.MonitorNode, string) (model.SwitchEvent, error)
}

func (m *Manager) SetAutoSwitch(ctx context.Context, revision int, enabled bool) (*model.MonitorPlan, error) {
	m.mu.Lock()
	p := clonePlan(m.plan)
	m.mu.Unlock()
	if p == nil {
		return nil, fmt.Errorf("请先保存监控方案")
	}
	return m.Save(ctx, model.MonitorRequest{Revision: revision, Enabled: p.Enabled, AutoSwitch: &enabled})
}

func fresh(at, now time.Time, age time.Duration) bool {
	return !at.IsZero() && !at.After(now) && now.Sub(at) <= age
}

// The election uses exactly the baseline success rate shown in the monitoring
// table. Long-term score and screening/refinement stages do not override it.
func failoverCandidates(o model.MonitorOverview) []model.MonitorRow {
	if o.Plan == nil || !o.Plan.Enabled || !o.Plan.AutoSwitch || o.Suspended || o.Issue != "" || !fresh(o.ObservedAt, o.Now, 45*time.Second) {
		return nil
	}
	failed := false
	for _, r := range o.Rows {
		if r.Name == o.Current && r.State.Status == "unavailable" && r.State.Failures >= 2 && fresh(r.State.LastAt, o.Now, 90*time.Second) {
			failed = true
		}
	}
	if !failed {
		return nil
	}
	out := []model.MonitorRow{}
	for _, r := range o.Rows {
		if r.Name != o.Current && r.State.Status == "healthy" && fresh(r.State.LastSuccess, o.Now, 240*time.Second) && fresh(r.State.LastAt, o.Now, 240*time.Second) && r.Metrics.Samples > 0 && r.Metrics.SuccessRate > 0 {
			out = append(out, r)
		}
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		if a.Metrics.SuccessRate != b.Metrics.SuccessRate {
			return a.Metrics.SuccessRate > b.Metrics.SuccessRate
		}
		if a.Metrics.P95MS != b.Metrics.P95MS {
			return a.Metrics.P95MS < b.Metrics.P95MS
		}
		return a.Name < b.Name
	})
	return out
}

func (m *Manager) failover(ctx context.Context, now time.Time) {
	source, ok := m.source.(switchSource)
	if !ok {
		return
	}
	m.mu.Lock()
	if m.closed || m.plan == nil || !m.plan.Enabled || !m.plan.AutoSwitch || m.fault || now.Before(m.failoverAt) {
		m.mu.Unlock()
		return
	}
	// Do not read history at all while the selected node is healthy/unknown.
	failed := false
	if m.issue == "" && fresh(m.observedAt, now, 45*time.Second) {
		for _, n := range m.plan.Nodes {
			state := m.states[n.ID]
			if n.Name == m.current && state.Status == "unavailable" && state.Failures >= 2 && fresh(state.LastAt, now, 90*time.Second) {
				failed = true
				break
			}
		}
	}
	if !failed {
		m.mu.Unlock()
		return
	}
	m.failoverAt = now.Add(30 * time.Second)
	m.mu.Unlock()
	o, err := m.overview(ctx, "24h", false)
	if err != nil {
		return
	}
	candidates := failoverCandidates(o)
	if len(candidates) == 0 {
		m.mu.Lock()
		if m.plan != nil && o.Plan != nil && m.plan.Revision == o.Plan.Revision {
			m.failoverMessage = "等待当前节点确认故障及可用候选；无符合条件的候选时保持现状"
		}
		m.mu.Unlock()
		return
	}
	for _, candidate := range candidates {
		m.mu.Lock()
		if m.closed || m.plan == nil || m.plan.Revision != o.Plan.Revision || !m.plan.AutoSwitch {
			m.mu.Unlock()
			return
		}
		profile := m.profile
		if len(profile.Probes) == 0 {
			m.mu.Unlock()
			return
		}
		child, cancel := context.WithCancel(ctx)
		m.inflight = cancel
		m.mu.Unlock()
		// Budgeted fresh verification never replaces a baseline sample.
		sample := model.MonitorSample{NodeID: candidate.ID, Kind: "failover", Slot: now.Unix() / 30, At: now, Outcome: "success"}
		for _, probe := range profile.Probes {
			if !m.takeBudget(m.now()) {
				sample.Outcome = "unknown"
				sample.Reason = "自动切换等待探测预算"
				sample.ReasonCode = "budget"
				break
			}
			delay, probeErr := m.source.MonitorDelay(child, candidate.MonitorNode, probe)
			if probeErr != nil || delay <= 0 {
				sample.Outcome = "failure"
				sample.Reason = "自动切换前复测失败"
				if errors.Is(probeErr, scan.ErrMonitorBusy) || (probeErr != nil && (strings.Contains(probeErr.Error(), "HTTP 401") || strings.Contains(probeErr.Error(), "HTTP 403"))) || !m.source.MonitorReachable(child) {
					sample.Outcome = "unknown"
					sample.Reason = "探测额度或 Controller 不可用，暂不切换"
					sample.ReasonCode = "controller"
					if errors.Is(probeErr, scan.ErrMonitorBusy) {
						sample.ReasonCode = "busy"
					}
				}
				break
			}
			sample.DelayMS = max(sample.DelayMS, delay)
		}
		sample.At = m.now()
		m.mu.Lock()
		if child.Err() != nil || m.closed || m.plan == nil || m.plan.Revision != o.Plan.Revision {
			m.mu.Unlock()
			cancel()
			return
		}
		previous := m.states[candidate.ID]
		state := transition(previous, sample)
		var event *model.MonitorEvent
		if previous.Status != state.Status {
			event = &model.MonitorEvent{NodeID: candidate.ID, NodeName: candidate.Name, At: sample.At, Status: state.Status, Message: sample.Reason}
		}
		added, writeErr := m.store.RecordMonitor(child, o.Plan.ID, sample, state, event)
		if writeErr != nil {
			m.fault = true
			m.issue = "自动切换前验证记录写入失败，已停止监控"
			m.mu.Unlock()
			cancel()
			return
		}
		if added {
			m.states[candidate.ID] = state
			m.dataVersion++
		}
		if sample.Outcome != "success" {
			m.failoverMessage = sample.Reason
			m.mu.Unlock()
			cancel()
			if sample.Outcome == "unknown" {
				return
			}
			continue
		}
		// Hold the plan lock until the audited action finishes: disabling or
		// editing a plan cannot race a switch already authorized under it.
		requestID := fmt.Sprintf("monitor:%s:%d:%s", o.Plan.ID, sample.At.UnixNano(), candidate.ID)
		result, switchErr := source.MonitorSwitch(child, *o.Plan, o.Current, candidate.MonitorNode, requestID)
		if switchErr != nil {
			m.failoverMessage = switchErr.Error()
		} else if result.Status == "confirmed" && result.AuditPersisted {
			m.dataVersion++
			m.current = candidate.Name
			m.refreshAt = time.Time{}
			m.failoverMessage = "已自动切换：" + o.Current + " → " + candidate.Name
		} else {
			m.failoverMessage = "自动切换结果未确认，请到选择历史核对后再试"
		}
		m.mu.Unlock()
		cancel()
		return
	}
}
