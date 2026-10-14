package scan

import (
	"context"
	"io"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/ipintel"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

const chatGPTTraceURL = "https://chatgpt.com/cdn-cgi/trace"

// MonitorEgress shares the foreground mutation gate. Only the dedicated probe
// selector is changed, and the source node is checked before accepting an IP.
func (m *Manager) MonitorEgress(ctx context.Context, group string, node model.MonitorNode, reserve func() bool) model.MonitorEgress {
	result := m.monitorTrace(ctx, group, node, reserve)
	if result.Status != "ok" || ctx.Err() != nil {
		return result
	}
	cfg := m.currentConfig().IPIntelligence
	if m.ipSettings != nil {
		cfg = m.ipSettings.Effective()
	}
	result.IntelligenceSources = m.ipIntelligence.LookupAll(ctx, result.IP, cfg, reserve)
	// Preserve the original IPQualityScore-only field for older consumers.
	for _, data := range result.IntelligenceSources {
		if data.Provider == "ipqualityscore" {
			copy := data.Clone()
			result.Intelligence = &copy
			break
		}
	}
	return result
}

func (m *Manager) monitorTrace(parent context.Context, group string, node model.MonitorNode, reserve func() bool) (result model.MonitorEgress) {
	result.Status, result.AttemptedAt = "not_configured", time.Now().UTC()
	if !m.settingsMu.TryLock() {
		result.Status = "busy"
		return
	}
	defer m.settingsMu.Unlock()
	cfg := m.currentConfig()
	if !cfg.EgressVerification.Enabled {
		return
	}
	if m.stopping || m.hasRunningScan() {
		result.Status = "busy"
		return
	}
	if group == cfg.EgressVerification.SelectorGroup {
		result.Status, result.Error = "error", "专用探测组不能作为业务监控目标"
		return
	}
	select {
	case m.probeSlots <- struct{}{}:
		defer func() { <-m.probeSlots }()
	default:
		result.Status = "busy"
		return
	}
	m.workers.Add(1)
	defer m.workers.Done()
	ctx, cancel := context.WithTimeout(parent, 12*time.Second)
	defer cancel()
	if !m.confirmEgressNode(ctx, group, node) {
		result.Status, result.Error = "error", "节点身份已变化，出口证据已丢弃"
		return
	}
	proxyURL, err := url.Parse(cfg.EgressVerification.ProxyURL)
	if err != nil || proxyURL.Host == "" || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") || proxyURL.User != nil {
		result.Status, result.Error = "error", "出口探测代理入口无效"
		return
	}
	// Reserve the trace request before any selector mutation.
	if !reserve() {
		result.Status = "budget"
		return
	}
	session, code := m.beginEgressProbeSelector(ctx, cfg.EgressVerification.SelectorGroup)
	if code != "" {
		result.Status, result.Error = "error", probeSelectionMessage(code)
		return
	}
	defer func() {
		if warning := session.restore("monitor-egress"); warning != nil {
			result = model.MonitorEgress{Status: "error", AttemptedAt: result.AttemptedAt, Error: warning.Message}
		}
	}()
	if code = session.selectCandidate(ctx, node.Name); code != "" {
		result.Status, result.Error = "error", probeSelectionMessage(code)
		return
	}
	transport := &http.Transport{Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true}
	defer transport.CloseIdleConnections()
	client := &http.Client{Timeout: 6 * time.Second, Transport: transport, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }}
	if m.monitorTraceTransport != nil {
		client.Transport = m.monitorTraceTransport(proxyURL)
	}
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, chatGPTTraceURL, nil)
	req.Header.Set("Cache-Control", "no-cache")
	resp, requestErr := client.Do(req)
	var body []byte
	var readErr error
	if requestErr == nil {
		body, readErr = io.ReadAll(io.LimitReader(resp.Body, (64<<10)+1))
		resp.Body.Close()
	}
	if code = session.confirmCandidate(ctx, node.Name); code != "" {
		result.Status, result.Error = "error", probeSelectionMessage(code)
		return
	}
	if !m.confirmEgressNode(ctx, group, node) {
		result.Status, result.Error = "error", "节点身份已变化，出口证据已丢弃"
		return
	}
	if requestErr != nil || readErr != nil || resp.StatusCode != http.StatusOK || len(body) > 64<<10 {
		result.Status, result.Error = "error", "ChatGPT 出口探测失败或响应无效"
		return
	}
	ip, country, ok := parseChatGPTTrace(string(body))
	if !ok {
		result.Status, result.Error = "error", "ChatGPT trace 未返回有效出口 IP"
		return
	}
	result.Status, result.IP, result.CountryCode, result.ObservedAt = "ok", ip, country, time.Now().UTC()
	return
}

// Bypass catalog caches: a node name can be reused by a different provider.
func (m *Manager) confirmEgressNode(ctx context.Context, group string, node model.MonitorNode) bool {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return false
	}
	g, exists := proxies[group]
	if !exists || !strings.EqualFold(g.Type, "Selector") || !contains(g.All, node.Name) {
		return false
	}
	providers, err := m.client.ListProviders(ctx)
	if err != nil {
		return false
	}
	if !matchesEgressIdentity(proxies, providers, node) {
		return false
	}
	if m.egressClient == nil {
		return true
	}
	probeProxies, err := m.egressClient.ListProxies(ctx)
	if err != nil {
		return false
	}
	probeProviders, err := m.egressClient.ListProviders(ctx)
	return err == nil && matchesEgressIdentity(probeProxies, probeProviders, node)
}

func matchesEgressIdentity(proxies map[string]mihomo.Proxy, providers []mihomo.Provider, node model.MonitorNode) bool {
	proxy, exists := proxies[node.Name]
	provider := ""
	for _, p := range providers {
		for _, candidate := range p.Proxies {
			if candidate.Name != node.Name {
				continue
			}
			if provider != "" && provider != p.Name {
				return false
			}
			provider = p.Name
			if !exists {
				proxy, exists = candidate, true
			}
		}
	}
	return exists && provider == node.Provider && proxy.Type == node.Protocol && !strings.EqualFold(proxy.Type, "Direct") && !strings.EqualFold(proxy.Type, "Reject") && !strings.EqualFold(proxy.Type, "Pass")
}

func parseChatGPTTrace(body string) (string, string, bool) {
	values := map[string]string{}
	for _, line := range strings.Split(body, "\n") {
		key, value, found := strings.Cut(strings.TrimSpace(line), "=")
		if found {
			if _, duplicate := values[key]; duplicate {
				return "", "", false
			}
			values[key] = value
		}
	}
	ip, ok := ipintel.PublicIP(values["ip"])
	country := strings.ToUpper(values["loc"])
	if len(country) != 2 || country[0] < 'A' || country[0] > 'Z' || country[1] < 'A' || country[1] > 'Z' {
		country = ""
	}
	return ip, country, ok
}
