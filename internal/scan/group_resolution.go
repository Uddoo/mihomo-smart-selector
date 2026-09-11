package scan

import (
	"sort"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func isLeafProxy(proxy mihomo.Proxy) bool {
	if len(proxy.All) > 0 || isPolicyGroup(proxy.Type) {
		return false
	}
	switch strings.ToLower(strings.ReplaceAll(proxy.Type, "-", "")) {
	case "direct", "reject", "rejectdrop", "pass", "compatible":
		return false
	default:
		return true
	}
}

func (m *Manager) emptyPreview(group mihomo.Proxy, proxies map[string]mihomo.Proxy, providers map[string]string, request model.ScanRequest, profile config.ProbeProfile) model.ScanPreview {
	preview := model.ScanPreview{Profile: m.profileSummary(profile), ReasonCode: "no_direct_leaves"}
	unfiltered := model.ScanRequest{TargetGroup: request.TargetGroup}
	if len(m.filterCandidates(group, proxies, providers, unfiltered)) > 0 {
		preview.ReasonCode = "filters_empty"
		preview.Reason = "此组有可扫描节点，但没有节点符合当前筛选。请清除地区或 Provider 筛选后重试。"
		return preview
	}
	preview.NestedSelectors = m.nestedSelectors(request.TargetGroup, proxies, providers, request)
	if len(preview.NestedSelectors) > 0 {
		preview.Reason = "此组通过下级策略组选择节点。请选择实际包含节点的下级 Selector，继续使用当前测试服务。"
	} else {
		preview.Reason = "此组没有可直接扫描的代理节点，也未找到符合筛选的下级 Selector。可清除筛选，或在 Mihomo 配置中添加直接引用节点或 Provider 的 select 组。"
	}
	return preview
}

// Resolve suggestions from the Mihomo graph, regardless of the GUI hosting it.
// Traversal is bounded and cycle-safe; it never writes or changes membership.
func (m *Manager) nestedSelectors(root string, proxies map[string]mihomo.Proxy, providers map[string]string, request model.ScanRequest) []model.NestedSelector {
	// A shared group can have a shorter inactive route. Resolve the selected
	// Selector chain separately so breadth-first deduplication cannot hide it.
	selectedPaths := map[string][]string{}
	for name, path := root, []string{root}; len(path) <= 12; {
		if _, seen := selectedPaths[name]; seen {
			break
		}
		selectedPaths[name] = append([]string(nil), path...)
		group, exists := proxies[name]
		if !exists || !strings.EqualFold(group.Type, "Selector") || group.Now == "" {
			break
		}
		member := false
		for _, child := range group.All {
			member = member || child == group.Now
		}
		if !member {
			break
		}
		name = group.Now
		path = append(path, name)
	}
	type visit struct {
		name    string
		path    []string
		current bool
	}
	queue := []visit{{root, []string{root}, true}}
	seen := map[string]bool{root: true}
	var options []model.NestedSelector
	for len(queue) > 0 && len(seen) <= 4096 && len(options) < 100 {
		item := queue[0]
		queue = queue[1:]
		group, exists := proxies[item.name]
		if !exists || len(item.path) > 12 {
			continue
		}
		if item.name != root && strings.EqualFold(group.Type, "Selector") && m.checkProbeTarget(item.name) == nil {
			if count := len(m.filterCandidates(group, proxies, providers, request)); count > 0 {
				if path, selected := selectedPaths[item.name]; selected {
					item.path, item.current = path, true
				}
				options = append(options, model.NestedSelector{Group: item.name, Path: item.path, CandidateCount: count, OnCurrentPath: item.current})
			}
		}
		// Relay members form a chain; a child alone is not that route.
		if strings.EqualFold(group.Type, "Relay") {
			continue
		}
		children := append([]string(nil), group.All...)
		sort.SliceStable(children, func(i, j int) bool {
			if (children[i] == group.Now) != (children[j] == group.Now) {
				return children[i] == group.Now
			}
			return children[i] < children[j]
		})
		for _, child := range children {
			proxy, exists := proxies[child]
			if !exists || seen[child] || !isPolicyGroup(proxy.Type) || len(seen) >= 4096 {
				continue
			}
			seen[child] = true
			path := append(append([]string(nil), item.path...), child)
			queue = append(queue, visit{child, path, item.current && strings.EqualFold(group.Type, "Selector") && group.Now == child})
		}
	}
	sort.SliceStable(options, func(i, j int) bool {
		return options[i].OnCurrentPath && !options[j].OnCurrentPath
	})
	return options
}
