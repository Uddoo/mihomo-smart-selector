package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Same mutex and audit store as manual selection. Only a fresh, expected
// Selector membership can be changed; pending/unknown outcomes block retries.
func (m *Manager) MonitorSwitch(ctx context.Context, p model.MonitorPlan, expected string, node model.MonitorNode, key string) (model.SwitchEvent, error) {
	var empty model.SwitchEvent
	if !m.settingsMu.TryLock() {
		return empty, fmt.Errorf("手动操作正在进行，自动切换稍后重试")
	}
	defer m.settingsMu.Unlock()
	ctx, actionCancel := context.WithTimeout(ctx, 8*time.Second)
	defer actionCancel()
	if !p.Enabled || !p.AutoSwitch || m.stopping || m.hasRunningScan() {
		return empty, fmt.Errorf("自动切换等待监控启用且手动扫描结束")
	}
	profile, err := m.currentConfig().ProbeProfileByID(p.ProfileID)
	if err != nil || MonitorProfileHash(profile) != p.ProfileHash {
		return empty, fmt.Errorf("服务模板已变化，未执行自动切换")
	}
	if profile.RequireStrict || profile.RequireRegion {
		return empty, fmt.Errorf("此模板要求严格/地区验证，当前自动切换证据不足，请手动验证")
	}
	if err = m.checkProbeTarget(p.Group); err != nil {
		return empty, err
	}
	if previous, err := m.store.SwitchByRequest(ctx, key); err == nil {
		if previous.ControllerScope != m.bindingScope() || previous.ScanID != "monitor:"+p.ID || previous.Group != p.Group || previous.Selected != node.Name || previous.Previous != expected {
			return empty, fmt.Errorf("自动切换请求标识已用于其他操作")
		}
		return previous, nil
	} else if !errors.Is(err, sql.ErrNoRows) {
		return empty, err
	}
	unresolved, err := m.store.UnresolvedSwitch(ctx, p.Group, m.bindingScope())
	if err != nil {
		return empty, err
	}
	if unresolved {
		return empty, fmt.Errorf("策略组有未确认切换，请先到选择历史核对")
	}
	last, err := m.store.LatestGroupSwitch(ctx, p.Group, m.bindingScope())
	if err != nil && !errors.Is(err, sql.ErrNoRows) {
		return empty, err
	}
	if err == nil && time.Since(last.CreatedAt) < 2*time.Minute {
		return empty, fmt.Errorf("切换冷却中：距离最近一次切换需满 2 分钟")
	}
	allowed := false
	for _, n := range p.Nodes {
		if model.SameMonitorNode(n, node) {
			allowed = true
		}
	}
	if !allowed || expected == node.Name {
		return empty, fmt.Errorf("目标不是此监控方案中的替代节点")
	}
	m.invalidateCatalog()
	_, nodes, current, err := m.MonitorCatalog(ctx, p.Group, p.ProfileID)
	if err != nil {
		return empty, err
	}
	present := false
	for _, n := range nodes {
		if model.SameMonitorNode(n, node) {
			present = true
		}
	}
	if !present || current != expected {
		return empty, fmt.Errorf("节点身份或当前选择已变化，已取消本次自动切换")
	}
	// Read immediately before durable intent; external Controller clients do
	// not support compare-and-swap, so readback remains mandatory afterwards.
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return empty, err
	}
	g, exists := proxies[p.Group]
	if !exists || !strings.EqualFold(g.Type, "Selector") || g.Now != expected || !contains(g.All, node.Name) {
		return empty, fmt.Errorf("策略组选择或成员已变化，未自动覆盖")
	}
	event := model.SwitchEvent{ControllerScope: m.bindingScope(), ScanID: "monitor:" + p.ID, Group: p.Group, Previous: expected, Selected: node.Name, Reason: "监控故障自动切换：候选按近24小时基准成功率优先，切换前复测通过", CreatedAt: time.Now().UTC(), Status: "pending", RequestID: key}
	event, err = m.store.RecordSwitch(ctx, event)
	if err != nil {
		return empty, fmt.Errorf("无法保存自动切换审计，未执行切换")
	}
	selectErr := m.client.Select(ctx, p.Group, node.Name)
	m.invalidateCatalog()
	verifyCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	return m.confirmSwitch(verifyCtx, event, selectErr)
}
