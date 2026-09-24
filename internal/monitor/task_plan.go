package monitor

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
	"time"
)

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
