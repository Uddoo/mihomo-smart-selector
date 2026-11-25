package scan

import (
	"context"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Empty in historical scan results. Primary evidence uses the local selector's
// readback. Isolated v1 additionally binds both endpoint catalogs to discovery.
const (
	egressSourcePrimary  = "primary"
	egressSourceIsolated = "isolated_identity_v1"
)

type egressProviderLeaf struct {
	owner string
	proxy mihomo.Proxy
}

// Discovery indexes every provider leaf once. A fresh single-candidate check
// retains only that name. A repeated name gets an unusable, ownerless entry.
func egressProviderLeaves(providers []mihomo.Provider, onlyName string) map[string]egressProviderLeaf {
	leaves := make(map[string]egressProviderLeaf)
	for _, provider := range providers {
		for _, p := range provider.Proxies {
			if onlyName != "" && p.Name != onlyName {
				continue
			}
			if _, duplicate := leaves[p.Name]; duplicate {
				leaves[p.Name] = egressProviderLeaf{}
			} else {
				leaves[p.Name] = egressProviderLeaf{owner: provider.Name, proxy: p}
			}
		}
	}
	return leaves
}

// Either catalog may be the sole source of a leaf. Every available assertion
// about its name, protocol and owner must agree, including provider-name on a
// /proxies entry when /providers/proxies has no entry for it.
func resolveEgressIdentity(proxies map[string]mihomo.Proxy, leaves map[string]egressProviderLeaf, name string) (model.MonitorNode, bool) {
	proxy, exists := proxies[name]
	owner := proxy.ProviderName
	if listing, listed := leaves[name]; listed {
		p := listing.proxy
		if strings.TrimSpace(listing.owner) == "" || !isLeafProxy(p) || (p.ProviderName != "" && p.ProviderName != listing.owner) {
			return model.MonitorNode{}, false
		}
		if exists && (proxy.Type != p.Type || (owner != "" && owner != listing.owner)) {
			return model.MonitorNode{}, false
		}
		owner = listing.owner
		if !exists {
			proxy, exists = p, true
		}
	}
	if !exists || name == "" || proxy.Name != name || strings.TrimSpace(proxy.Type) == "" || !isLeafProxy(proxy) {
		return model.MonitorNode{}, false
	}
	return model.MonitorNode{Name: name, Provider: owner, Protocol: proxy.Type}, true
}

func freezeEgressIdentities(candidates []candidate, read catalogRead) {
	if read.providerErr != nil {
		return
	}
	leaves := egressProviderLeaves(read.providers, "")
	for i := range candidates {
		c := &candidates[i]
		node, ok := resolveEgressIdentity(read.proxies, leaves, c.Name)
		if ok && node.Provider == c.Provider {
			c.Protocol = node.Protocol
		}
	}
}

func matchesEgressIdentity(proxies map[string]mihomo.Proxy, providers []mihomo.Provider, node model.MonitorNode) bool {
	fresh, ok := resolveEgressIdentity(proxies, egressProviderLeaves(providers, node.Name), node.Name)
	return ok && fresh.Protocol == node.Protocol && fresh.Provider == node.Provider
}

type isolatedEgressCandidates struct {
	businessGroup string
	nodes         map[string]model.MonitorNode
}

func newIsolatedEgressCandidates(group string, candidates []candidate) *isolatedEgressCandidates {
	nodes := make(map[string]model.MonitorNode, len(candidates))
	for _, c := range candidates {
		if c.Protocol != "" {
			nodes[c.Name] = model.MonitorNode{Name: c.Name, Provider: c.Provider, Protocol: c.Protocol}
		}
	}
	return &isolatedEgressCandidates{businessGroup: group, nodes: nodes}
}

func (s *probeSelectorSession) readCandidateSelector(ctx context.Context, name string, isolated *isolatedEgressCandidates) (mihomo.Proxy, string) {
	if isolated == nil {
		return readProbeSelectorUsing(ctx, s.client, s.group)
	}
	node, ok := isolated.nodes[name]
	if !ok || !s.manager.confirmBusinessEgressNode(ctx, isolated.businessGroup, node) {
		return mihomo.Proxy{}, "egress_identity_unconfirmed"
	}
	providers, err := s.client.ListProviders(ctx)
	if err != nil {
		return mihomo.Proxy{}, "egress_identity_unconfirmed"
	}
	// Read the selector last, so an intervening change during the other three
	// catalog requests cannot be overwritten. The same snapshot proves the leaf.
	group, proxies, code := readProbeSelectorCatalog(ctx, s.client, s.group)
	if code != "" {
		return mihomo.Proxy{}, code
	}
	if group.Name != s.group || !matchesEgressIdentity(proxies, providers, node) {
		return mihomo.Proxy{}, "egress_identity_unconfirmed"
	}
	return group, ""
}
