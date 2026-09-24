package scan

import (
	"context"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"sort"
	"strings"
)

func (m *Manager) Groups(ctx context.Context) ([]mihomo.Proxy, error) {
	proxies, err := m.catalogProxies(ctx)
	if err != nil {
		return nil, err
	}
	groups := make([]mihomo.Proxy, 0)
	for _, proxy := range proxies {
		if strings.EqualFold(proxy.Type, "Selector") {
			groups = append(groups, proxy)
		}
	}
	sort.Slice(groups, func(left, right int) bool { return groups[left].Name < groups[right].Name })
	return groups, nil
}

func (m *Manager) Providers(ctx context.Context) ([]mihomo.Provider, error) {
	providers, err := m.catalogProviders(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(providers, func(left, right int) bool { return providers[left].Name < providers[right].Name })
	return providers, nil
}

func (m *Manager) Regions() []config.Region {
	return m.classifier.Regions()
}

func (m *Manager) Nodes(ctx context.Context) ([]model.NodeSummary, error) {
	proxies, err := m.catalogProxies(ctx)
	if err != nil {
		return nil, err
	}
	providers, err := m.catalogProviders(ctx)
	if err != nil {
		return nil, err
	}
	nodes := make([]model.NodeSummary, 0)
	seen := map[string]bool{}
	for _, provider := range providers {
		for _, proxy := range provider.Proxies {
			if proxy.Name == "" || seen[proxy.Name] || isPolicyGroup(proxy.Type) {
				continue
			}
			seen[proxy.Name] = true
			nodes = append(nodes, m.nodeSummary(proxy.Name, provider.Name, proxy.Type))
		}
	}
	for name, proxy := range proxies {
		if name == "" || seen[name] || isPolicyGroup(proxy.Type) {
			continue
		}
		seen[name] = true
		nodes = append(nodes, m.nodeSummary(name, proxy.ProviderName, proxy.Type))
	}
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Name < nodes[right].Name })
	return nodes, nil
}

func (m *Manager) nodeSummary(name, provider, protocol string) model.NodeSummary {
	kind, match := m.classifier.ClassifyEntry(name, protocol)
	return model.NodeSummary{Name: name, Provider: provider, Protocol: protocol,
		InferredRegion: match.Code, RegionSource: match.Source, EntryKind: kind,
		RegionReason: match.Reason, RegionCandidates: match.Candidates, RegionEvidence: match.Evidence}
}

func (m *Manager) History(ctx context.Context, limit int) ([]model.SwitchEvent, error) {
	if limit < 1 {
		limit = 20
	}
	if limit > 100 {
		limit = 100
	}
	return m.store.ListSwitches(ctx, limit)
}
