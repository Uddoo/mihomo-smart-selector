package scan

import (
	"context"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Preflight is a read-only preview. It deliberately does not take the scan
// admission gate: holding it across two Controller round-trips made every
// filter change queue behind the previous preview. Only the resolved settings
// snapshot is read here, and Start re-validates everything under the gate
// before a scan can actually begin.
func (m *Manager) Preflight(ctx context.Context, request model.ScanRequest) (model.ScanPreview, error) {
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
	group, proxies, providerByNode, err := m.preflightMembership(ctx, request)
	if err != nil {
		return model.ScanPreview{}, err
	}
	candidates := m.filterCandidates(group, proxies, providerByNode, request)
	if len(candidates) == 0 {
		return m.emptyPreview(group, proxies, providerByNode, request, profile), nil
	}
	// Read the settings snapshot once; currentConfig copies the whole Config.
	scanner := m.currentConfig().Scanner
	if len(candidates) > scanner.MaxTotalCandidates {
		return model.ScanPreview{}, fmt.Errorf("%d candidates exceed scanner.max_total_candidates (%d); narrow regions or providers", len(candidates), scanner.MaxTotalCandidates)
	}
	samples := max(2, scanner.Samples)
	if request.Mode == "quick" {
		samples = 1
	}
	refineCount := 0
	if request.Mode == "stable" {
		refineCount = min(len(candidates), scanner.RefineTopK+1)
		if len(request.Nodes) > 0 {
			refineCount = len(candidates)
		}
	}
	batchSize := scanner.BatchSize
	return model.ScanPreview{
		RefineCandidates: refineCount,
		CandidateCount:   len(candidates), BatchSize: batchSize,
		BatchCount:     (len(candidates)+batchSize-1)/batchSize + (refineCount+batchSize-1)/batchSize,
		SamplesPerNode: samples, ProbeRequests: (len(candidates) + refineCount*(samples-1)) * len(profile.Probes),
		Profile: m.profileSummary(profile), Ready: true,
	}, nil
}
