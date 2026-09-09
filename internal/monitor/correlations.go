package monitor

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"sort"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func correlationObservations(p model.MonitorPlan, states map[string]model.MonitorState, issue string, now time.Time) []model.MonitorCorrelation {
	by := map[string][]model.MonitorNode{}
	healthy := map[string]bool{}
	for _, n := range p.Nodes {
		if n.Provider != "" {
			by[n.Provider] = append(by[n.Provider], n)
		}
		state := states[n.ID]
		if state.Status == "healthy" && fresh(state.LastAt, now, 240*time.Second) {
			healthy[n.Provider] = true
		}
	}
	out := []model.MonitorCorrelation{}
	for provider, nodes := range by {
		o := model.MonitorCorrelation{PlanID: p.ID, Provider: provider, UpdatedAt: now, Monitored: len(nodes), Nodes: []string{}, Reliable: issue == ""}
		signature := []any{issue}
		for _, n := range nodes {
			state := states[n.ID]
			signature = append(signature, n.ID, state.Status, state.LastAt.UnixNano())
			if !fresh(state.LastAt, now, 240*time.Second) {
				continue
			}
			if state.Status == "healthy" || state.Status == "unavailable" {
				o.Comparable++
			}
			if state.Status == "unavailable" {
				o.Failed++
				o.Nodes = append(o.Nodes, n.Name)
			}
		}
		for other, ok := range healthy {
			if other != provider && other != "" && ok {
				o.OtherProviderHealthy = true
			}
		}
		if o.OtherProviderHealthy {
			o.Message = "该 Provider 所监控候选疑似共同异常；另一个 Provider 有同服务成功观测"
		} else {
			o.Message = "所监控候选出现共同异常，但缺少跨 Provider 成功对照，不能区分共同上游与本地环境"
		}
		signature = append(signature, o.OtherProviderHealthy, o.Comparable)
		bytes, _ := json.Marshal(signature)
		sum := sha256.Sum256(bytes)
		o.Signature = hex.EncodeToString(sum[:])
		sort.Strings(o.Nodes)
		out = append(out, o)
	}
	return out
}

func (m *Manager) correlate(ctx context.Context, now time.Time) {
	m.mu.Lock()
	if now.Before(m.correlationAt) {
		m.mu.Unlock()
		return
	}
	m.correlationAt = now.Add(30 * time.Second)
	p := clonePlan(m.plan)
	states := map[string]model.MonitorState{}
	for k, v := range m.states {
		states[k] = v
	}
	issue := m.issue
	if !fresh(m.observedAt, now, 45*time.Second) {
		issue = "metadata stale"
	}
	if m.fault {
		issue = "storage"
	}
	m.mu.Unlock()
	if p == nil {
		return
	}
	scope := m.source.MonitorScope()
	if err := m.store.EndMonitorCorrelations(ctx, scope, p.ID, !p.Enabled, now); err != nil {
		m.storageFailure()
		return
	}
	if !p.Enabled {
		return
	}
	for _, o := range correlationObservations(*p, states, issue, now) {
		if err := m.store.ObserveMonitorCorrelation(ctx, scope, o); err != nil {
			m.storageFailure()
			return
		}
	}
}

func (m *Manager) Storage(ctx context.Context) (model.MonitorStorage, error) {
	return m.store.MonitorStorage(ctx, m.source.MonitorScope())
}
func (m *Manager) SaveRetention(ctx context.Context, p model.MonitorRetention) (model.MonitorRetention, error) {
	saved, err := m.store.SaveMonitorRetention(ctx, m.source.MonitorScope(), p)
	if err == nil {
		m.mu.Lock()
		m.dataVersion++
		m.cleanAt = time.Time{}
		m.mu.Unlock()
	}
	return saved, err
}
func (m *Manager) Correlations(ctx context.Context) ([]model.MonitorCorrelation, error) {
	return m.store.MonitorCorrelations(ctx, m.source.MonitorScope())
}
