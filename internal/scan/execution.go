package scan

import (
	"context"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"maps"
	"strings"
	"time"
)

func (m *Manager) run(ctx context.Context, scan model.Scan) {
	results, warnings, err := m.scan(ctx, scan)
	scan.Warnings = warnings
	completed := time.Now().UTC()
	scan.Progress = m.progressSnapshot(scan.ID)
	if err == errScanStopped || err == context.Canceled {
		scan.Status = model.ScanCancelled
		scan.Error = "scan stopped before final ranking"
		for index := range results {
			// Partial results have no final score ordering, but they still need a
			// stable unique storage key when the cancelled scan is persisted.
			results[index].Rank = index + 1
		}
		scan.Results = results
	} else if err != nil {
		scan.Status = model.ScanFailed
		scan.Error = err.Error()
	} else {
		scan.Status = model.ScanComplete
		scan.Results = results
	}
	scan.CompletedAt = &completed
	if persistErr := m.store.CompleteScan(context.Background(), scan); persistErr != nil {
		scan.Status = model.ScanFailed
		scan.Error = "could not persist scan results; selection is unavailable"
		m.setActive(scan)
		m.publish(scan.ID, Event{Kind: "error", Message: "scan completed but could not persist results", At: completed})
		return
	}
	m.removeActive(scan.ID)
	kind, message := "completed", "scan completed"
	if err != nil {
		kind, message = "error", "scan failed: "+err.Error()
	}
	m.publish(scan.ID, Event{Kind: kind, Message: message, At: completed})
}

func (m *Manager) scan(ctx context.Context, scan model.Scan) ([]model.NodeResult, []model.ScanWarning, error) {
	profile, err := m.profileFor(scan.Request)
	if err != nil {
		return nil, nil, err
	}
	candidates, err := m.discoverCandidates(ctx, scan.Request)
	if err != nil {
		return nil, nil, err
	}
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return nil, nil, fmt.Errorf("discover proxies for egress verification: %w", err)
	}
	if len(candidates) > m.currentConfig().Scanner.MaxTotalCandidates {
		return nil, nil, fmt.Errorf("%d candidates exceed scanner.max_total_candidates (%d); narrow regions or providers", len(candidates), m.currentConfig().Scanner.MaxTotalCandidates)
	}

	allResults, err := m.stagedProbes(ctx, scan, profile, candidates, proxies[scan.Request.TargetGroup].Now)
	if err != nil {
		return allResults, nil, err
	}
	rankNodes(allResults)

	var warnings []model.ScanWarning
	recordWarning := func(warning *model.ScanWarning) {
		if warning == nil {
			return
		}
		warnings = append(warnings, *warning)
		m.mu.Lock()
		if active := m.active[scan.ID]; active != nil {
			active.Warnings = append(active.Warnings, *warning)
		}
		m.mu.Unlock()
		m.publish(scan.ID, Event{Kind: "warning", Message: warning.Message, At: time.Now().UTC()})
	}
	if m.currentConfig().EgressVerification.Enabled {
		recordWarning(m.verifyEgress(ctx, allResults, scan.ID))
		for index := range allResults {
			calculateMetrics(&allResults[index], m.currentConfig().Scanner)
		}
	}
	strict := m.currentConfig().Scanner.StrictVerification
	if len(warnings) > 0 && strict.Enabled && warnings[0].Group == strict.SelectorGroup {
		// An unresolved cleanup must not be hidden by reusing the same group
		// and treating its temporary selection as a new restoration baseline.
		for index := range allResults {
			allResults[index].StrictVerificationStatus = "probe_verification_stopped"
			allResults[index].RestrictionStatus = "not_checked"
		}
	} else {
		recordWarning(m.verifyStrict(ctx, profile, allResults, scan.ID))
	}
	if err := ctx.Err(); err != nil {
		return allResults, warnings, err
	}
	for index := range allResults {
		assessResult(&allResults[index], profile, m.currentConfig().Scanner.StrictVerification.Enabled)
	}
	rankNodes(allResults)
	return allResults, warnings, nil
}

func (m *Manager) discoverCandidates(ctx context.Context, request model.ScanRequest) ([]candidate, error) {
	group, proxies, providerByNode, err := m.discoverMembership(ctx, request)
	if err != nil {
		return nil, err
	}
	candidates := m.filterCandidates(group, proxies, providerByNode, request)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w matched the requested filters", errNoCandidates)
	}
	return candidates, nil
}

func (m *Manager) discoverMembership(ctx context.Context, request model.ScanRequest) (mihomo.Proxy, map[string]mihomo.Proxy, map[string]string, error) {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return mihomo.Proxy{}, nil, nil, fmt.Errorf("discover proxies: %w", err)
	}
	// Provider enrichment belongs to this scan, not the controller's shared snapshot.
	proxies = maps.Clone(proxies)
	group, exists := proxies[request.TargetGroup]
	if !exists || !strings.EqualFold(group.Type, "Selector") {
		return mihomo.Proxy{}, nil, nil, fmt.Errorf("target group %q is not a Mihomo Selector", request.TargetGroup)
	}
	providerByNode := map[string]string{}
	providers, providerErr := m.client.ListProviders(ctx)
	if providerErr == nil {
		for _, provider := range providers {
			for _, proxy := range provider.Proxies {
				providerByNode[proxy.Name] = provider.Name
				// Some controller builds list provider-owned leaf nodes only under
				// /providers/proxies, while selector.all still references them by
				// name. Merge that metadata so those legitimate members remain
				// scannable; selector membership is revalidated before selection.
				if _, exists := proxies[proxy.Name]; !exists {
					proxy.ProviderName = provider.Name
					proxies[proxy.Name] = proxy
				}
			}
		}
	} else if len(request.Providers) > 0 {
		return mihomo.Proxy{}, nil, nil, fmt.Errorf("discover providers for selected provider filter: %w", providerErr)
	}
	return group, proxies, providerByNode, nil
}
