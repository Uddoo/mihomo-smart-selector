package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

var ErrMonitorBusy = errors.New("扫描正在使用探测额度")

func (m *Manager) MonitorScope() string { return m.bindingScope() }

func MonitorProfileHash(p config.ProbeProfile) string {
	data, _ := json.Marshal(p.Probes)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}

func (m *Manager) MonitorCatalog(ctx context.Context, group, profileID string) (config.ProbeProfile, []model.MonitorNode, string, error) {
	p, err := m.currentConfig().ProbeProfileByID(profileID)
	if err != nil || p.RequiresConfiguration || len(p.Probes) == 0 {
		return p, nil, "", fmt.Errorf("请选择已配置 HTTPS 探测的服务模板")
	}
	proxies, err := m.catalogProxies(ctx)
	if err != nil {
		return p, nil, "", fmt.Errorf("Controller 不可达，监控数据未知")
	}
	g, ok := proxies[group]
	if !ok || !strings.EqualFold(g.Type, "Selector") {
		return p, nil, "", fmt.Errorf("目标策略组已失效")
	}
	providers, err := m.catalogProviders(ctx)
	if err != nil {
		return p, nil, g.Now, fmt.Errorf("无法核实 Provider 身份，暂停节点探测")
	}
	by := map[string]string{}
	ambiguous := map[string]bool{}
	for _, provider := range providers {
		for _, node := range provider.Proxies {
			if previous, exists := by[node.Name]; exists && previous != provider.Name {
				ambiguous[node.Name] = true
			}
			by[node.Name] = provider.Name
			if _, ok := proxies[node.Name]; !ok {
				node.ProviderName = provider.Name
				proxies[node.Name] = node
			}
		}
	}
	candidates := m.filterCandidates(g, proxies, by, model.ScanRequest{TargetGroup: group})
	out := make([]model.MonitorNode, 0, len(candidates))
	for _, c := range candidates {
		protocol := proxies[c.Name].Type
		// Do not silently attach history to whichever duplicate provider was
		// encountered first in a map iteration, or treat DIRECT as a proxy node.
		if ambiguous[c.Name] || strings.EqualFold(protocol, "Direct") || strings.EqualFold(protocol, "Reject") || strings.EqualFold(protocol, "Pass") {
			continue
		}
		data, _ := json.Marshal([]string{m.bindingScope(), c.Provider, c.Name, protocol})
		sum := sha256.Sum256(data)
		out = append(out, model.MonitorNode{ID: hex.EncodeToString(sum[:16]), Name: c.Name, Provider: c.Provider, Protocol: protocol})
	}
	return p, out, g.Now, nil
}

// The same gate is used by foreground scans. A monitor never queues behind
// a saturated scan: that time slot is recorded as unknown, not node failure.
func (m *Manager) MonitorDelay(ctx context.Context, node model.MonitorNode, probe config.Probe) (int, error) {
	m.settingsMu.Lock()
	if m.stopping {
		m.settingsMu.Unlock()
		return 0, ErrMonitorBusy
	}
	slots := m.probeSlots
	select {
	case slots <- struct{}{}:
		m.settingsMu.Unlock()
		defer func() { <-slots }()
	default:
		m.settingsMu.Unlock()
		return 0, ErrMonitorBusy
	}
	child, cancel := context.WithTimeout(ctx, 5500*time.Millisecond)
	defer cancel()
	return m.client.Delay(child, node.Name, node.Provider, probe, 5000)
}

func (m *Manager) MonitorReachable(ctx context.Context) bool {
	_, err := m.client.Reachable(ctx)
	return err == nil
}

func (m *Manager) MonitorForegroundBusy() bool { return m.hasRunningScan() }
