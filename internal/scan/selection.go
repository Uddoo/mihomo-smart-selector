package scan

import (
	"context"
	"database/sql"
	"errors"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"strings"
	"time"
)

func (m *Manager) Select(ctx context.Context, scanID, requestedNode string) (model.SwitchEvent, error) {
	return m.SelectRequest(ctx, scanID, requestedNode, newID())
}

func (m *Manager) SelectRequest(ctx context.Context, scanID, requestedNode, requestID string) (model.SwitchEvent, error) {
	if len(requestID) < 1 || len(requestID) > 128 {
		return model.SwitchEvent{}, fmt.Errorf("request_id must contain 1..128 characters")
	}
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	existing, err := m.store.SwitchByRequest(ctx, requestID)
	if err == nil {
		if existing.ControllerScope != m.bindingScope() {
			return model.SwitchEvent{}, fmt.Errorf("切换记录不属于当前 Controller，请连接原 Controller 后核对")
		}
		if existing.ScanID != scanID || (requestedNode != "" && existing.Selected != requestedNode) {
			return model.SwitchEvent{}, fmt.Errorf("request_id belongs to a different selection")
		}
		return existing, nil
	}
	if !errors.Is(err, sql.ErrNoRows) {
		return model.SwitchEvent{}, err
	}
	if m.stopping {
		return model.SwitchEvent{}, fmt.Errorf("服务正在停止")
	}
	scan, err := m.store.GetScan(ctx, scanID)
	if err != nil {
		return model.SwitchEvent{}, err
	}
	if scan.Status != model.ScanComplete {
		return model.SwitchEvent{}, fmt.Errorf("scan %q is not complete", scanID)
	}
	var selected *model.NodeResult
	for index := range scan.Results {
		if requestedNode == "" && scan.Results[index].Rank == 1 {
			selected = &scan.Results[index]
			break
		}
		if requestedNode != "" && scan.Results[index].Name == requestedNode {
			selected = &scan.Results[index]
			break
		}
	}
	if selected == nil {
		return model.SwitchEvent{}, fmt.Errorf("requested node is not a result of scan %q", scanID)
	}
	profile, err := m.currentConfig().ProbeProfileByID(scan.Request.ProfileID)
	if err != nil {
		return model.SwitchEvent{}, fmt.Errorf("服务模板已失效，请重新扫描")
	}
	if reason := m.selectionReason(scan, *selected, profile, time.Now()); reason != "" {
		return model.SwitchEvent{}, fmt.Errorf("%s", reason)
	}
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return model.SwitchEvent{}, err
	}
	group, exists := proxies[scan.Request.TargetGroup]
	if !exists || !strings.EqualFold(group.Type, "Selector") {
		return model.SwitchEvent{}, fmt.Errorf("target group %q is no longer a selector", scan.Request.TargetGroup)
	}
	if !contains(group.All, selected.Name) {
		return model.SwitchEvent{}, fmt.Errorf("node %q is no longer a member of target group", selected.Name)
	}
	// Fresh discovery can take time; enforce expiry again immediately before PUT.
	if reason := m.selectionReason(scan, *selected, profile, time.Now()); reason != "" {
		return model.SwitchEvent{}, fmt.Errorf("%s", reason)
	}
	unresolved, err := m.store.UnresolvedSwitch(ctx, group.Name, m.bindingScope())
	if err != nil {
		return model.SwitchEvent{}, err
	}
	if unresolved {
		return model.SwitchEvent{}, fmt.Errorf("该策略组有未确认操作，请先在选择历史中核对结果")
	}
	event := model.SwitchEvent{ControllerScope: m.bindingScope(), ScanID: scan.ID, Group: group.Name, Previous: group.Now, Selected: selected.Name, Reason: "manual selection pending", CreatedAt: time.Now().UTC(), Status: "pending", RequestID: requestID}
	event, err = m.store.RecordSwitch(ctx, event)
	if err != nil {
		return model.SwitchEvent{}, fmt.Errorf("无法保存待执行审计，未执行切换: %w", err)
	}
	selectErr := m.client.Select(ctx, group.Name, selected.Name)
	m.invalidateCatalog()
	verifyCtx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	return m.confirmSwitch(verifyCtx, event, selectErr)
}

func (m *Manager) confirmSwitch(ctx context.Context, event model.SwitchEvent, selectErr error) (model.SwitchEvent, error) {
	proxies, err := m.client.ListProxies(ctx)
	group, exists := proxies[event.Group]
	switch {
	case err != nil || !exists || !strings.EqualFold(group.Type, "Selector"):
		event.Status = "unknown"
		event.Reason = "无法回读确认当前选择，请核对结果；不要重复提交切换"
	case group.Now == event.Selected:
		event.Status = "confirmed"
		event.Reason = "已回读确认目标节点"
	case selectErr != nil:
		event.Status = "unknown"
		event.Reason = "切换请求异常且当前节点与目标不符，请核对结果"
	default:
		event.Status = "failed"
		event.Reason = "回读时当前节点与目标不符"
	}
	// Persist independently of a cancelled browser request or expired readback.
	saveCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err := m.store.FinalizeSwitch(saveCtx, event); err != nil {
		event.AuditPersisted = false
		event.Reason += "；审计更新失败，历史保留为待核对状态"
	} else {
		event.AuditPersisted = true
	}
	return event, nil
}

func (m *Manager) ReconcileSwitch(ctx context.Context, id int64) (model.SwitchEvent, error) {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	event, err := m.store.SwitchByID(ctx, id)
	if err != nil {
		return event, err
	}
	if event.Status != "pending" && event.Status != "unknown" {
		return event, nil
	}
	if event.ControllerScope == "" || event.ControllerScope != m.bindingScope() {
		return event, fmt.Errorf("切换记录不属于当前 Controller，请连接原 Controller 后核对")
	}
	// A deliberate reconciliation records current observed state, not a replay.
	return m.confirmSwitch(ctx, event, nil)
}
