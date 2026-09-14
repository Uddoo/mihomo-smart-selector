package scan

import (
	"context"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Controller reads are observations, not a lock against external clients.
// Track only the last confirmed selection and the last attempted write so
// cleanup does not knowingly overwrite a different client's selection.
type probeSelectorSession struct {
	manager     *Manager
	group       string
	previous    string
	expected    string
	lastAttempt string
}

func (m *Manager) readProbeSelector(ctx context.Context, name string) (mihomo.Proxy, string) {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return mihomo.Proxy{}, "probe_selector_unconfirmed"
	}
	group, ok := proxies[name]
	if !ok || !strings.EqualFold(group.Type, "Selector") {
		return mihomo.Proxy{}, "probe_selector_unavailable"
	}
	return group, ""
}

func (m *Manager) beginProbeSelector(ctx context.Context, name string) (*probeSelectorSession, string) {
	group, code := m.readProbeSelector(ctx, name)
	if code != "" {
		return nil, code
	}
	if group.Now == "" || !contains(group.All, group.Now) {
		return nil, "probe_selector_unconfirmed"
	}
	return &probeSelectorSession{manager: m, group: name, previous: group.Now, expected: group.Now}, ""
}

func (s *probeSelectorSession) selectCandidate(ctx context.Context, name string) string {
	group, code := s.manager.readProbeSelector(ctx, s.group)
	if code != "" {
		return code
	}
	if group.Now != s.expected {
		return "probe_selector_changed"
	}
	if !contains(group.All, name) {
		return "candidate_not_in_probe_selector"
	}
	s.lastAttempt = name // A failed response may still have applied the write.
	if err := s.manager.client.Select(ctx, s.group, name); err != nil {
		return "probe_selector_switch_failed"
	}
	if code := s.confirmCandidate(ctx, name); code != "" {
		return code
	}
	s.expected = name
	return ""
}

func (s *probeSelectorSession) confirmCandidate(ctx context.Context, name string) string {
	group, code := s.manager.readProbeSelector(ctx, s.group)
	if code != "" {
		return code
	}
	if group.Now != name || !contains(group.All, name) {
		return "probe_selector_changed"
	}
	return ""
}

func (s *probeSelectorSession) restore(phase string) *model.ScanWarning {
	if s.lastAttempt == "" {
		return nil
	}
	warning := func(code, message string) *model.ScanWarning {
		return &model.ScanWarning{Code: code, Phase: phase, Group: s.group, Message: message}
	}
	unconfirmed := func() *model.ScanWarning {
		return warning("probe_restore_unconfirmed", "无法确认探测组已恢复，请到 Controller 核对当前选择。")
	}
	// Cleanup must still run after cancellation, with its own bounded lifetime.
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	group, code := s.manager.readProbeSelector(ctx, s.group)
	if code != "" {
		return unconfirmed()
	}
	if group.Now == s.previous {
		return nil
	}
	if group.Now != s.expected && group.Now != s.lastAttempt {
		return warning("probe_restore_conflict", "探测组已被其他操作改选，已跳过恢复，请核对当前选择。")
	}
	if !contains(group.All, s.previous) {
		return warning("probe_restore_failed", "未能恢复探测组，请到 Controller 核对当前选择。")
	}
	// Readback determines success even if the write response was lost.
	selectErr := s.manager.client.Select(ctx, s.group, s.previous)
	group, code = s.manager.readProbeSelector(ctx, s.group)
	if code != "" {
		return unconfirmed()
	}
	if group.Now != s.previous {
		if selectErr != nil {
			return warning("probe_restore_failed", "恢复探测组请求失败，请到 Controller 核对当前选择。")
		}
		return warning("probe_restore_failed", "未能恢复探测组，请到 Controller 核对当前选择。")
	}
	return nil
}

func probeSelectionMessage(code string) string {
	switch code {
	case "candidate_not_in_probe_selector":
		return "候选未加入专用选择器"
	case "probe_selector_unavailable":
		return "专用选择器不可用"
	case "probe_selector_switch_failed":
		return "专用选择器切换失败"
	case "probe_selector_changed":
		return "探测期间选择发生变化，验证证据已丢弃"
	default:
		return "无法确认专用选择器当前节点，验证已停止"
	}
}
