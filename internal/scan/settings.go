package scan

import (
	"context"
	"encoding/json"
	"fmt"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) hasRunningScan() bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	for _, s := range m.active {
		if s.Status == model.ScanRunning {
			return true
		}
	}
	return false
}

func (m *Manager) checkProbeTarget(target string) error {
	c := m.currentConfig()
	if (c.EgressVerification.Enabled && target == c.EgressVerification.SelectorGroup) || (c.Scanner.StrictVerification.Enabled && target == c.Scanner.StrictVerification.SelectorGroup) {
		return fmt.Errorf("专用探测组不能作为业务扫描目标")
	}
	return nil
}

func (m *Manager) LoadSettings(ctx context.Context) error {
	data, err := m.store.RuntimeSettings(ctx, m.bindingScope())
	if err != nil || len(data) == 0 {
		return err
	}
	s := m.currentConfig().RuntimeSettings()
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	cfg, err := m.currentConfig().WithRuntimeSettings(s)
	if err != nil {
		return fmt.Errorf("saved runtime settings: %w", err)
	}
	m.cfg.Store(&cfg)
	m.probeSlots = make(chan struct{}, cfg.Scanner.Concurrency)
	m.settingsRevision = s.Revision
	return nil
}

func (m *Manager) Settings() config.RuntimeSettings {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	s := m.currentConfig().RuntimeSettings()
	s.Revision = m.settingsRevision
	return s
}

func (m *Manager) SaveSettings(ctx context.Context, s config.RuntimeSettings) (config.RuntimeSettings, error) {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	if m.hasRunningScan() {
		return s, fmt.Errorf("扫描进行中，请结束扫描后保存设置")
	}
	if s.Revision != m.settingsRevision {
		return s, fmt.Errorf("设置已被其他页面更新，请重新加载后再保存")
	}
	next, err := m.currentConfig().WithRuntimeSettings(s)
	if err != nil {
		return s, err
	}
	if next.Scanner.Concurrency != cap(m.probeSlots) && len(m.probeSlots) > 0 {
		return s, fmt.Errorf("后台监控正在探测，请稍后重试更改并发数，或先暂停监控")
	}
	for _, v := range []struct {
		enabled bool
		group   string
	}{{s.Egress.Enabled, s.Egress.SelectorGroup}, {s.Strict.Enabled, s.Strict.SelectorGroup}} {
		if !v.enabled {
			continue
		}
		proxies, err := m.client.ListProxies(ctx)
		if err != nil {
			return s, fmt.Errorf("无法检查探测组，请确认 Controller 连接")
		}
		group, ok := proxies[v.group]
		if !ok || !strings.EqualFold(group.Type, "Selector") {
			return s, fmt.Errorf("专用探测组 %q 不存在或不是 Selector，请先在 OpenClash 配置", v.group)
		}
	}
	s.Revision++
	data, err := json.Marshal(s)
	if err != nil {
		return s, err
	}
	if err := m.store.SaveRuntimeSettings(ctx, m.bindingScope(), data); err != nil {
		return s, fmt.Errorf("无法保存运行设置")
	}
	m.cfg.Store(&next)
	if cap(m.probeSlots) != next.Scanner.Concurrency {
		m.probeSlots = make(chan struct{}, next.Scanner.Concurrency)
	}
	m.settingsRevision = s.Revision
	return s, nil
}
