package scan

import (
	"context"
	"errors"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// ValidateMonitorScan checks a ranked scan prefix against current monitor candidates.
func (m *Manager) ValidateMonitorScan(ctx context.Context, scanID, group, profileID string, nodes []string) error {
	scan, err := m.store.GetScan(ctx, scanID)
	if err != nil {
		return err
	}
	if scan.ControllerScope == "" || scan.ControllerScope != m.bindingScope() {
		return errors.New("扫描不属于当前 Controller，请重新扫描")
	}
	if scan.Status != model.ScanComplete {
		return errors.New("扫描完成后可替换监控列表。")
	}
	if group != scan.Request.TargetGroup || len(nodes) == 0 || len(nodes) > len(scan.Results) {
		return errors.New("扫描排名与提交名单不一致，请重新读取扫描。")
	}
	results := append([]model.NodeResult(nil), scan.Results...)
	rankNodes(results)
	for i, name := range nodes {
		if name != results[i].Name {
			return errors.New("扫描排名与提交名单不一致，请重新读取扫描。")
		}
	}
	_, candidates, _, err := m.MonitorCatalog(ctx, group, profileID)
	if err != nil {
		return err
	}
	byName := make(map[string]string, len(candidates))
	for _, node := range candidates {
		byName[node.Name] = node.Provider
	}
	for _, result := range results[:len(nodes)] {
		if provider, ok := byName[result.Name]; !ok || provider != result.Provider {
			return errors.New("排名中的节点已失效，请重新扫描后替换。")
		}
	}
	return nil
}
