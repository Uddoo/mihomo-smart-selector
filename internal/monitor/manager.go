package monitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"strings"
	"sync"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

type Source interface {
	MonitorScope() string
	MonitorCatalog(context.Context, string, string) (config.ProbeProfile, []model.MonitorNode, string, error)
	MonitorDelay(context.Context, model.MonitorNode, config.Probe) (int, error)
	MonitorReachable(context.Context) bool
}

type TaskRuntime struct {
	owner              *Manager
	saveMu             sync.Mutex
	mu                 sync.Mutex
	dataVersion        uint64
	instanceID         string
	store              *history.Store
	queries            *queryBudget
	source             Source
	plan               *model.MonitorPlan
	profile            config.ProbeProfile
	catalog            map[string]bool
	current, issue     string
	failoverMessage    string
	failoverAt         time.Time
	correlationAt      time.Time
	observedAt         time.Time
	fault              bool
	states             map[string]model.MonitorState
	manual             map[string]bool
	slots              map[string]int64
	refreshAt, cleanAt time.Time
	inflight           context.CancelFunc
	closed             bool
	now                func() time.Time
}

func newTask(owner *Manager, p *model.MonitorPlan) (*TaskRuntime, error) {
	m := &TaskRuntime{owner: owner, store: owner.storeRef, source: owner.sourceRef, plan: p, queries: newQueryBudget(), states: map[string]model.MonitorState{}, manual: map[string]bool{}, slots: map[string]int64{}, instanceID: owner.instance, now: owner.clock}
	m.queries.slots = owner.historySlots
	if p != nil {
		if p.CandidateLimit == 0 {
			p.CandidateLimit = Limits().DefaultCandidateLimit
		}
		var err error
		m.states, m.slots, err = m.store.MonitorCheckpoint(context.Background(), p.Nodes)
		if err != nil {
			return nil, err
		}
		owner.budget.set(p.TaskID, requestBudget(len(p.Nodes), 1), p.Enabled)
	}
	return m, nil
}

func sameNodes(a, b []model.MonitorNode) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !model.SameMonitorNode(a[i], b[i]) {
			return false
		}
	}
	return true
}

func clonePlan(p *model.MonitorPlan) *model.MonitorPlan {
	if p == nil {
		return nil
	}
	v := *p
	v.Nodes = append([]model.MonitorNode(nil), p.Nodes...)
	return &v
}

func (m *TaskRuntime) Save(ctx context.Context, r model.MonitorRequest) (*model.MonitorPlan, error) {
	return m.savePlan(ctx, r, false)
}

func (m *TaskRuntime) savePlan(ctx context.Context, r model.MonitorRequest, implicit bool) (*model.MonitorPlan, error) {
	m.saveMu.Lock()
	defer m.saveMu.Unlock()
	m.mu.Lock()
	old := clonePlan(m.plan)
	closed := m.closed
	m.mu.Unlock()
	if closed {
		return nil, fmt.Errorf("服务正在停止")
	}
	var configuredProfile *config.ProbeProfile
	commit := func(p *model.MonitorPlan, previous int) (*model.MonitorPlan, error) {
		return m.commitPlan(ctx, p, previous, implicit, configuredProfile)
	}
	previous := 0
	if old != nil {
		previous = old.Revision
	}
	if r.Revision != previous {
		return nil, fmt.Errorf("监控配置已变化，请刷新后重试")
	}
	limit := Limits().DefaultCandidateLimit
	if old != nil {
		limit = old.CandidateLimit
		if limit == 0 {
			limit = Limits().DefaultCandidateLimit
		}
	}
	if r.CandidateLimit != nil {
		limit = *r.CandidateLimit
	}
	if limit < 1 || limit > Limits().MaxCandidateLimit {
		return nil, fmt.Errorf("监控候选上限必须为 1–30 的整数")
	}
	// Pausing must also work while the Controller or profile is unavailable.
	if !r.Enabled && (implicit || r.Group == "") {
		if old == nil {
			return nil, fmt.Errorf("尚未创建监控")
		}
		if len(old.Nodes) > limit {
			return nil, fmt.Errorf("已选节点超过候选上限，请减少节点或提高上限")
		}
		old.CandidateLimit = limit
		old.Enabled = false
		if r.AutoSwitch != nil {
			old.AutoSwitch = *r.AutoSwitch
		}
		old.Revision++
		return commit(old, previous)
	}
	if r.Group == "" && old != nil {
		r.Group = old.Group
		r.ProfileID = old.ProfileID
		for _, n := range old.Nodes {
			r.Nodes = append(r.Nodes, n.Name)
		}
	}
	p, nodes, _, err := m.source.MonitorCatalog(ctx, r.Group, r.ProfileID)
	if err != nil {
		return nil, err
	}
	if len(p.Probes) < 1 || len(p.Probes) > Limits().MaxProbeCount {
		return nil, fmt.Errorf("监控模板须包含 1–6 个探测目标，请调整服务模板")
	}
	if len(r.Nodes) < 1 || len(r.Nodes) > limit {
		return nil, fmt.Errorf("请选择至少 1 个叶子节点，且不超过候选上限")
	}
	selected := []model.MonitorNode{}
	seen := map[string]bool{}
	for _, name := range r.Nodes {
		if seen[name] {
			return nil, fmt.Errorf("监控节点不能重复")
		}
		seen[name] = true
		found := false
		for _, n := range nodes {
			if n.Name == name {
				selected = append(selected, n)
				found = true
				break
			}
		}
		if !found {
			return nil, fmt.Errorf("候选已失效或不在目标组内，请刷新")
		}
	}
	hash := scan.MonitorProfileHash(p)
	configuredProfile = &p
	plan := &model.MonitorPlan{Enabled: r.Enabled, CandidateLimit: limit, Group: r.Group, ProfileID: p.ID, ProfileHash: hash, Nodes: selected, Revision: previous + 1, CreatedAt: m.now()}
	if old != nil {
		plan.TaskID = old.TaskID
	}
	if old != nil && old.Group == plan.Group {
		plan.AutoSwitch = old.AutoSwitch
	}
	if r.AutoSwitch != nil {
		plan.AutoSwitch = *r.AutoSwitch
	}
	if old != nil && old.Group == plan.Group && old.ProfileID == plan.ProfileID && old.ProfileHash == hash && sameNodes(old.Nodes, selected) {
		plan.ID = old.ID
		plan.CreatedAt = old.CreatedAt
	} else {
		var bytes [16]byte
		if _, err = rand.Read(bytes[:]); err != nil {
			return nil, err
		}
		plan.ID = hex.EncodeToString(bytes[:])
	}
	if plan.TaskID == "" {
		plan.TaskID = plan.ID
	}
	return commit(plan, previous)
}

func (m *TaskRuntime) commitPlan(ctx context.Context, p *model.MonitorPlan, previous int, implicit bool, profile *config.ProbeProfile) (*model.MonitorPlan, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.owner.closing.Load() {
		return nil, fmt.Errorf("服务正在停止")
	}
	if (m.plan == nil && previous != 0) || (m.plan != nil && m.plan.Revision != previous) {
		return nil, fmt.Errorf("监控配置已变化，请刷新后重试")
	}
	p.UpdatedAt = m.now()
	if err := m.store.PrepareMonitorPlan(ctx, m.source.MonitorScope(), p); err != nil {
		return nil, err
	}
	var restoredStates map[string]model.MonitorState
	var restoredSlots map[string]int64
	changed := m.plan == nil || m.plan.ID != p.ID
	if changed {
		var e error
		restoredStates, restoredSlots, e = m.store.MonitorCheckpoint(ctx, p.Nodes)
		if e != nil {
			return nil, e
		}
	}
	save := m.store.SaveMonitorTask
	if implicit {
		save = m.store.SaveMonitorPlan
	}
	if err := save(ctx, m.source.MonitorScope(), *p, previous); err != nil {
		return nil, err
	}
	if m.inflight != nil {
		m.inflight()
	}
	if changed {
		m.states = restoredStates
		m.slots = restoredSlots
	}
	m.plan = clonePlan(p)
	if profile != nil {
		m.profile = *profile
	}
	m.dataVersion++
	m.manual = map[string]bool{}
	m.refreshAt = time.Time{}
	m.current = ""
	m.issue = ""
	if p.Enabled {
		m.fault = false
		m.owner.storageFault.Store(false)
	}
	m.owner.budget.set(p.TaskID, requestBudget(len(p.Nodes), len(m.profile.Probes)), p.Enabled)
	m.failoverMessage = ""
	m.failoverAt = time.Time{}
	return clonePlan(p), nil
}

func (m *TaskRuntime) Retest(node string, revision int) error {
	m.mu.Lock()
	defer m.mu.Unlock()
	if m.closed || m.owner.closing.Load() || m.plan == nil || !m.plan.Enabled || m.fault || m.owner.storageFault.Load() {
		return fmt.Errorf("请先启动监控")
	}
	if m.plan.Revision != revision {
		return fmt.Errorf("监控配置已变化，请刷新")
	}
	for _, n := range m.plan.Nodes {
		if n.ID == node {
			m.manual[node] = true
			return nil
		}
	}
	return fmt.Errorf("节点不在当前监控中")
}

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
