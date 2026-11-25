package monitor

import (
	"context"
	"fmt"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type switchSource interface {
	MonitorSwitch(context.Context, model.MonitorPlan, string, model.MonitorNode, string) (model.SwitchEvent, error)
}

func (m *TaskRuntime) SetAutoSwitch(ctx context.Context, revision int, enabled bool) (*model.MonitorPlan, error) {
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

func confirmedFailure(state model.MonitorState, now time.Time) bool {
	return state.Status == "unavailable" && state.Failures >= 2 && fresh(state.LastAt, now, 90*time.Second)
}

func freshBackup(state model.MonitorState, now time.Time) bool {
	return state.Status == "healthy" && fresh(state.LastSuccess, now, 240*time.Second) && fresh(state.LastAt, now, 240*time.Second)
}

const noFailoverCandidate = "等待当前节点确认故障及可用候选；无符合条件的候选时保持现状"

// Use one best-rate anchor for the whole election. Pairwise tolerances would
// make ordering non-transitive. Within the preferred band, prioritize P95;
// keep every other eligible node as a success-rate-ordered fallback.
func failoverCandidates(o model.MonitorOverview) []model.MonitorRow {
	if o.Plan == nil || !o.Plan.Enabled || !o.Plan.AutoSwitch || o.Suspended || o.Issue != "" || !fresh(o.ObservedAt, o.Now, 45*time.Second) {
		return nil
	}
	failed := false
	for _, r := range o.Rows {
		if r.Name == o.Current && confirmedFailure(r.State, o.Now) {
			failed = true
		}
	}
	if !failed {
		return nil
	}
	out := []model.MonitorRow{}
	bestRate := 0.0
	for _, r := range o.Rows {
		if r.Name != o.Current && freshBackup(r.State, o.Now) && r.Metrics.Samples > 0 && r.Metrics.SuccessRate > 0 {
			out = append(out, r)
			bestRate = max(bestRate, r.Metrics.SuccessRate)
		}
	}
	preferred := func(r model.MonitorRow) bool {
		if o.Plan.FailoverTolerancePP == 0 {
			return r.Metrics.SuccessRate == bestRate
		}
		// Epsilon only absorbs floating-point error at an inclusive boundary.
		return bestRate-r.Metrics.SuccessRate <= o.Plan.FailoverTolerancePP/100+1e-12
	}
	sort.SliceStable(out, func(i, j int) bool {
		a, b := out[i], out[j]
		aPreferred, bPreferred := preferred(a), preferred(b)
		if aPreferred != bPreferred {
			return aPreferred
		}
		if aPreferred && a.Metrics.P95MS != b.Metrics.P95MS {
			return a.Metrics.P95MS < b.Metrics.P95MS
		}
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

func (m *TaskRuntime) failover(ctx context.Context, now time.Time) {
	source, ok := m.source.(switchSource)
	if !ok {
		return
	}
	m.mu.Lock()
	if ctx.Err() != nil || m.closed || m.plan == nil || !m.plan.Enabled || !m.plan.AutoSwitch || m.fault || m.owner.storageFault.Load() || now.Before(m.failoverAt) {
		m.mu.Unlock()
		return
	}
	if m.issue != "" || !fresh(m.observedAt, now, 45*time.Second) {
		m.mu.Unlock()
		return
	}
	failed, hasBackup := false, false
	for _, n := range m.plan.Nodes {
		state := m.states[n.ID]
		if n.Name == m.current {
			if !confirmedFailure(state, now) {
				m.mu.Unlock()
				return
			}
			failed = true
		} else if freshBackup(state, now) {
			hasBackup = true
		}
		if failed && hasBackup {
			break
		}
	}
	if !failed {
		m.mu.Unlock()
		return
	}
	m.failoverAt = now.Add(30 * time.Second)
	if !hasBackup {
		m.failoverMessage = noFailoverCandidate
		m.mu.Unlock()
		return
	}
	m.mu.Unlock()
	o, err := m.overview(ctx, "24h", false)
	if err != nil {
		return
	}
	candidates := failoverCandidates(o)
	if len(candidates) == 0 {
		m.mu.Lock()
		if m.plan != nil && o.Plan != nil && m.plan.Revision == o.Plan.Revision {
			m.failoverMessage = noFailoverCandidate
		}
		m.mu.Unlock()
		return
	}
	for _, candidate := range candidates {
		m.mu.Lock()
		if ctx.Err() != nil || m.closed || m.plan == nil || m.plan.Revision != o.Plan.Revision || !m.plan.AutoSwitch || m.owner.storageFault.Load() {
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
		sample := model.MonitorSample{NodeID: candidate.ID, Kind: "failover", Slot: now.Unix() / 30, At: now}
		if !m.probeNode(child, candidate.MonitorNode, profile, &sample) {
			cancel()
			return
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
			if child.Err() != nil {
				m.mu.Unlock()
				cancel()
				return
			}
			m.owner.storageFault.Store(true)
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
		if m.owner.storageFault.Load() {
			m.mu.Unlock()
			cancel()
			return
		}
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
