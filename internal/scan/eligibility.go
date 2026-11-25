package scan

import (
	"context"
	"fmt"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Resolve once per scan view or selection, not once per ranked result.
type selectionPolicy struct {
	requireStrict, requireRegion bool
	strictRulesID                string
	expectedRegions              []string
}

func selectionPolicyFor(p config.ProbeProfile) selectionPolicy {
	policy := selectionPolicy{requireStrict: p.RequireStrict, requireRegion: p.RequireRegion, expectedRegions: p.ExpectedRegions}
	if p.RequireStrict {
		policy.strictRulesID = strictRulesID(p.StrictProbes)
	}
	return policy
}

func (m *Manager) selectionReason(s model.Scan, r model.NodeResult, policy selectionPolicy, now time.Time) string {
	if s.ControllerScope == "" || s.ControllerScope != m.bindingScope() {
		return "扫描不属于当前 Controller，请重新扫描"
	}
	if err := m.checkProbeTarget(s.Request.TargetGroup); err != nil {
		return err.Error()
	}
	if s.Status != model.ScanComplete {
		return "扫描完成后可选择"
	}
	if s.Request.Mode == "stable" && r.Stage != "refined" {
		return "此节点仅完成初筛，请复测后选择"
	}
	measured := r.MeasuredAt
	if measured.IsZero() && s.CompletedAt != nil {
		measured = *s.CompletedAt
	}
	if measured.IsZero() || !now.Before(measured.Add(time.Duration(m.currentConfig().Scanner.ResultMaxAgeSeconds)*time.Second)) {
		return "扫描结果已过期，请复测此节点后重新确认"
	}
	if r.SuccessRate < m.currentConfig().Scanner.MinSuccessRate {
		return "成功率低于最低切换门槛"
	}
	if policy.requireStrict {
		if r.StrictVerificationStatus != "passed" {
			return "此服务要求严格验证通过"
		}
		if s.Profile.StrictRulesID == "" || policy.strictRulesID == "" {
			return "严格规则版本未知，请重新扫描"
		}
		if s.Profile.StrictRulesID != policy.strictRulesID {
			return "严格验证规则已变化，请重新扫描"
		}
	}
	if policy.requireRegion {
		if r.VerifiedRegion == "" || !containsFold(policy.expectedRegions, r.VerifiedRegion) {
			return "此服务要求出口地区匹配"
		}
		// A legacy country alone cannot tell us whether a different Controller's
		// same-name leaf supplied it. Keep it visible, but require new evidence.
		if r.EgressEvidenceSource != egressSourcePrimary && r.EgressEvidenceSource != egressSourceIsolated {
			return "已返回的数据保留供查看；重新扫描完成后可选择节点。"
		}
	}
	return ""
}

func (m *Manager) decorateScan(s model.Scan) model.Scan {
	p, err := m.currentConfig().ProbeProfileByID(s.Request.ProfileID)
	policy := selectionPolicyFor(p)
	for i := range s.Results {
		r := &s.Results[i]
		measured := r.MeasuredAt
		if measured.IsZero() && s.CompletedAt != nil {
			measured = *s.CompletedAt
		}
		if !measured.IsZero() {
			r.ExpiresAt = measured.Add(time.Duration(m.currentConfig().Scanner.ResultMaxAgeSeconds) * time.Second)
		}
		if err != nil {
			r.SelectionReason = "服务模板已失效，请重新扫描"
		} else {
			r.SelectionReason = m.selectionReason(s, *r, policy, time.Now())
		}
	}
	return s
}

// Retesting creates a new scan, never a switch. The user must inspect its
// evidence and explicitly select again after it completes.
func (m *Manager) Retest(ctx context.Context, id, node string) (model.Scan, error) {
	s, err := m.store.GetScan(ctx, id)
	if err != nil {
		return model.Scan{}, err
	}
	if s.ControllerScope == "" || s.ControllerScope != m.bindingScope() {
		return model.Scan{}, fmt.Errorf("扫描不属于当前 Controller，请重新扫描")
	}
	if s.Status != model.ScanComplete {
		return model.Scan{}, fmt.Errorf("只能复测已完成扫描的节点")
	}
	found := false
	for _, r := range s.Results {
		if r.Name == node {
			found = true
			break
		}
	}
	if !found {
		return model.Scan{}, fmt.Errorf("节点不属于该次扫描")
	}
	return m.Start(model.ScanRequest{TargetGroup: s.Request.TargetGroup, ProfileID: s.Request.ProfileID, Mode: "stable", Nodes: []string{node}})
}
