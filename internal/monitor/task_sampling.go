package monitor

import (
	"context"
	"errors"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
	"strings"
	"time"
)

func (m *TaskRuntime) refresh(ctx context.Context, plan model.MonitorPlan, now time.Time) {
	probeCtx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	p, nodes, current, err := m.source.MonitorCatalog(probeCtx, plan.Group, plan.ProfileID)
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || ctx.Err() != nil || m.plan == nil || m.plan.Revision != plan.Revision {
		return
	}
	previousIssue := m.issue
	defer func() {
		if previousIssue != m.issue {
			status := "recovered"
			message := "Controller 与监控配置已恢复可读"
			if m.issue != "" {
				status = "unknown"
				message = m.issue
			}
			writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
			defer cancel()
			if err := m.store.RecordMonitorSystem(writeCtx, m.source.MonitorScope(), plan.ID, status, message, now); err != nil && ctx.Err() == nil {
				m.fault = true
				m.owner.storageFault.Store(true)
				m.issue = "环境事件写入失败，监控已暂停"
			}
		}
	}()
	m.dataVersion++
	m.refreshAt = now.Add(30 * time.Second)
	m.observedAt = now
	m.current = current
	m.catalog = map[string]bool{}
	if err != nil {
		m.issue = err.Error()
		m.owner.budget.set(plan.TaskID, 0, false)
		return
	}
	if scan.MonitorProfileHash(p) != plan.ProfileHash {
		m.issue = "服务探测配置已变化，请重新保存监控，开始新的评分记录"
		m.owner.budget.set(plan.TaskID, 0, false)
		return
	}
	m.profile = p
	m.owner.budget.set(plan.TaskID, requestBudget(len(plan.Nodes), len(p.Probes)), plan.Enabled)
	m.issue = ""
	for _, n := range nodes {
		m.catalog[n.ID] = true
	}
}

func (m *TaskRuntime) step(ctx context.Context, now time.Time) {
	m.mu.Lock()
	p := clonePlan(m.plan)
	refresh := !now.Before(m.refreshAt)
	fault := m.fault || m.owner.storageFault.Load()
	var stop context.CancelFunc
	if p != nil && p.Enabled && !fault && !m.closed {
		ctx, stop = context.WithCancel(ctx)
		m.inflight = stop
	}
	m.mu.Unlock()
	if stop == nil {
		return
	}
	defer stop()
	if refresh {
		m.refresh(ctx, *p, now)
	}
	m.mu.Lock()
	if m.closed || m.plan == nil || m.plan.Revision != p.Revision {
		m.mu.Unlock()
		return
	}
	// Select one due item each tick. Baselines have priority over extra checks.
	var chosen *model.MonitorNode
	kind := ""
	slot := int64(0)
	for i, n := range p.Nodes {
		anchor := nodeAnchor(*p, i)
		if now.Unix() < anchor {
			continue
		}
		k := (now.Unix() - anchor) / Interval
		// After a restart or a long blocked tick, do not replay missed slots.
		if now.Unix()-(anchor+k*Interval) > 15 {
			continue
		}
		key := n.ID + "/baseline"
		if last, ok := m.slots[key]; ok && last >= k {
			continue
		}
		m.slots[key] = k
		v := n
		chosen = &v
		kind = "baseline"
		slot = k
		break
	}
	if chosen == nil {
		for _, n := range p.Nodes {
			s := m.states[n.ID]
			elapsed := now.Sub(s.LastAt)
			if m.manual[n.ID] {
				delete(m.manual, n.ID)
				kind = "manual"
			} else if (s.Status == "suspect" || s.Status == "recovering") && elapsed >= 10*time.Second {
				kind = "confirmation"
			} else if n.Name == m.current && elapsed >= 30*time.Second {
				kind = "current"
			} else {
				continue
			}
			slot = now.Unix() / 30
			key := n.ID + "/" + kind
			if last, ok := m.slots[key]; ok && last >= slot {
				continue
			}
			m.slots[key] = slot
			v := n
			chosen = &v
			break
		}
	}
	if chosen == nil {
		m.mu.Unlock()
		return
	}
	profile := m.profile
	issue := m.issue
	present := m.catalog[chosen.ID]
	child, cancel := context.WithCancel(ctx)
	m.inflight = cancel
	m.mu.Unlock()
	defer cancel()
	sample := model.MonitorSample{NodeID: chosen.ID, Kind: kind, Slot: slot, At: now, Outcome: "unknown"}
	if issue != "" {
		sample.Reason = issue
		sample.ReasonCode = "environment"
	} else if !present {
		sample.Reason = "节点已移除或 Provider/协议身份发生变化"
		sample.ReasonCode = "node_missing"
	} else {
		sample.Outcome = "success"
		for _, probe := range profile.Probes {
			if !m.takeBudget(m.now()) {
				sample.Outcome = "unknown"
				sample.Reason = "已达到后台请求预算，本次未完成"
				sample.ReasonCode = "budget"
				break
			}
			delay, err := m.source.MonitorDelay(child, *chosen, probe)
			if child.Err() != nil {
				return
			}
			if err != nil {
				if errors.Is(err, scan.ErrMonitorBusy) {
					sample.Outcome = "unknown"
					sample.Reason = "手动扫描占用探测额度，本次漏测"
					sample.ReasonCode = "busy"
				} else if strings.Contains(err.Error(), "HTTP 401") || strings.Contains(err.Error(), "HTTP 403") || !m.source.MonitorReachable(child) {
					sample.Outcome = "unknown"
					sample.Reason = "Controller 无法完成请求，未归因于节点"
					sample.ReasonCode = "controller"
				} else {
					sample.Outcome = "failure"
					sample.Reason = "节点路径探测失败或超时；未定位具体故障位置"
				}
				break
			}
			sample.DelayMS = max(sample.DelayMS, delay)
		}
	}
	sample.At = m.now()
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.plan == nil || m.plan.Revision != p.Revision || child.Err() != nil {
		return
	}
	prev := m.states[chosen.ID]
	state := transition(prev, sample)
	var event *model.MonitorEvent
	if state.Status != prev.Status {
		event = &model.MonitorEvent{NodeID: chosen.ID, NodeName: chosen.Name, At: sample.At, Status: state.Status, Message: sample.Reason}
		if sample.Outcome == "success" {
			event.Message = "HTTPS 探测成功；长连接未验证"
		}
	}
	added, err := m.store.RecordMonitor(child, p.ID, sample, state, event)
	if err != nil {
		if child.Err() != nil {
			return
		}
		m.fault = true
		m.owner.storageFault.Store(true)
		m.dataVersion++
		m.issue = "监控写入失败，已停止采样；修复存储后点击继续监控"
		return
	}
	if added {
		m.states[chosen.ID] = state
		m.dataVersion++
	}
}

func (m *TaskRuntime) takeBudget(now time.Time) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.plan == nil || !m.plan.Enabled || m.closed || m.owner.storageFault.Load() {
		return false
	}
	m.owner.budget.set(m.plan.TaskID, requestBudget(len(m.plan.Nodes), len(m.profile.Probes)), true)
	return m.owner.budget.take(m.plan.TaskID, now)
}

func (m *TaskRuntime) storageFailure() {
	m.mu.Lock()
	defer m.mu.Unlock()
	m.storageFailureLocked()
}

func (m *TaskRuntime) storageFailureLocked() {
	m.fault = true
	m.owner.storageFault.Store(true)
	m.dataVersion++
	m.issue = "监控存储维护失败，已停止采样；修复存储后点击继续监控"
}
