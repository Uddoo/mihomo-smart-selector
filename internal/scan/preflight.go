package scan

import (
	"context"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) Preflight(ctx context.Context, request model.ScanRequest) (model.ScanPreview, error) {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	if err := m.checkProbeTarget(request.TargetGroup); err != nil {
		return model.ScanPreview{}, err
	}
	request, err := normaliseRequest(request)
	if err != nil {
		return model.ScanPreview{}, err
	}
	profile, err := m.resolveService(ctx, request)
	if err != nil {
		return model.ScanPreview{}, err
	}
	if profile.RequiresConfiguration {
		return model.ScanPreview{Profile: m.profileSummary(profile)}, nil
	}
	if len(profile.Probes) == 0 {
		return model.ScanPreview{}, fmt.Errorf("probe profile %q has no reachable probes", profile.Label)
	}
	group, proxies, providerByNode, err := m.discoverMembership(ctx, request)
	if err != nil {
		return model.ScanPreview{}, err
	}
	candidates := m.filterCandidates(group, proxies, providerByNode, request)
	if len(candidates) == 0 {
		return m.emptyPreview(group, proxies, providerByNode, request, profile), nil
	}
	if len(candidates) > m.currentConfig().Scanner.MaxTotalCandidates {
		return model.ScanPreview{}, fmt.Errorf("%d candidates exceed scanner.max_total_candidates (%d); narrow regions or providers", len(candidates), m.currentConfig().Scanner.MaxTotalCandidates)
	}
	samples := max(2, m.currentConfig().Scanner.Samples)
	if request.Mode == "quick" {
		samples = 1
	}
	refineCount := 0
	if request.Mode == "stable" {
		refineCount = min(len(candidates), m.currentConfig().Scanner.RefineTopK+1)
		if len(request.Nodes) > 0 {
			refineCount = len(candidates)
		}
	}
	batchSize := m.currentConfig().Scanner.BatchSize
	return model.ScanPreview{
		RefineCandidates: refineCount,
		CandidateCount:   len(candidates), BatchSize: m.currentConfig().Scanner.BatchSize,
		BatchCount:     (len(candidates)+batchSize-1)/batchSize + (refineCount+batchSize-1)/batchSize,
		SamplesPerNode: samples, ProbeRequests: (len(candidates) + refineCount*(samples-1)) * len(profile.Probes),
		Profile: m.profileSummary(profile), Ready: true,
	}, nil
}
