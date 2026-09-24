package scan

import (
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"strings"
)

func normaliseRequest(request model.ScanRequest) (model.ScanRequest, error) {
	request.TargetGroup = strings.TrimSpace(request.TargetGroup)
	if request.TargetGroup == "" {
		return model.ScanRequest{}, fmt.Errorf("target_group is required")
	}
	if request.Mode == "" {
		request.Mode = "stable"
	}
	if request.Mode != "stable" && request.Mode != "quick" {
		return model.ScanRequest{}, fmt.Errorf("mode must be stable or quick")
	}
	request.Nodes = normaliseStrings(request.Nodes)
	request.Regions = normaliseCodes(request.Regions)
	request.Providers = normaliseStrings(request.Providers)
	return request, nil
}

func normaliseCodes(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value == "" {
			continue
		}
		value = strings.ToUpper(value)
		if !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func normaliseStrings(values []string) []string {
	result := make([]string, 0, len(values))
	seen := map[string]bool{}
	for _, value := range values {
		value = strings.TrimSpace(value)
		if value != "" && !seen[value] {
			seen[value] = true
			result = append(result, value)
		}
	}
	return result
}

func contains(values []string, wanted string) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func containsFold(values []string, wanted string) bool {
	for _, value := range values {
		if strings.EqualFold(strings.TrimSpace(value), strings.TrimSpace(wanted)) {
			return true
		}
	}
	return false
}

func containsStatus(values []int, wanted int) bool {
	for _, value := range values {
		if value == wanted {
			return true
		}
	}
	return false
}

func isPolicyGroup(proxyType string) bool {
	switch strings.ToLower(strings.ReplaceAll(proxyType, "-", "")) {
	case "selector", "urltest", "fallback", "loadbalance", "smart", "relay":
		return true
	default:
		return false
	}
}
