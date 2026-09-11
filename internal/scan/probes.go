package scan

import (
	"context"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"sync"
	"time"
)

func (m *Manager) probeBatch(parent context.Context, scan model.Scan, profile config.ProbeProfile, candidates []candidate) []model.NodeResult {
	jobs := make(chan candidate)
	results := make(chan model.NodeResult, len(candidates))
	var workers sync.WaitGroup
	for worker := 0; worker < m.currentConfig().Scanner.Concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				results <- m.probeCandidate(parent, item, profile, scan.Request.Mode)
			}
		}()
	}
	go func() {
		defer func() {
			close(jobs)
			workers.Wait()
			close(results)
		}()
		for _, item := range candidates {
			select {
			case <-parent.Done():
				return
			case jobs <- item:
			}
		}
	}()

	batchResults := make([]model.NodeResult, 0, len(candidates))
	for result := range results {
		copy, progress := m.recordCandidate(scan.ID, result, profile)
		m.publish(scan.ID, Event{Kind: "candidate-complete", Message: "candidate tested: " + result.Name, At: time.Now().UTC(), Result: &copy, Progress: &progress})
		batchResults = append(batchResults, result)
	}
	return batchResults
}

func (m *Manager) filterCandidates(group mihomo.Proxy, proxies map[string]mihomo.Proxy, providerByNode map[string]string, request model.ScanRequest) []candidate {
	seen := map[string]bool{}
	items := make([]candidate, 0, len(group.All))
	for _, name := range group.All {
		if len(request.Nodes) > 0 && !contains(request.Nodes, name) {
			continue
		}
		if seen[name] {
			continue
		}
		seen[name] = true
		proxy, exists := proxies[name]
		if !exists {
			continue
		}
		if !isLeafProxy(proxy) {
			continue
		}
		provider := proxy.ProviderName
		if provider == "" {
			provider = providerByNode[name]
		}
		region := m.classifier.Classify(name)
		if len(request.Regions) > 0 && !contains(request.Regions, region.Code) {
			continue
		}
		if len(request.Providers) > 0 && !contains(request.Providers, provider) {
			continue
		}
		items = append(items, candidate{Name: name, Provider: provider, Region: region})
	}
	return items
}

func (m *Manager) probeCandidate(parent context.Context, item candidate, profile config.ProbeProfile, mode string) model.NodeResult {
	result := model.NodeResult{
		Name: item.Name, Provider: item.Provider, InferredRegion: item.Region.Code, RegionSource: item.Region.Source, Stage: "screened",
	}
	select {
	case m.probeSlots <- struct{}{}:
		defer func() { <-m.probeSlots }()
	case <-parent.Done():
		return result
	}
	samples := max(2, m.currentConfig().Scanner.Samples)
	if mode == "refine" {
		samples--
	}
	if mode == "quick" {
		samples = 1
	}
sampling:
	for _, probe := range profile.Probes {
		for sample := 0; sample < samples; sample++ {
			if parent.Err() != nil {
				break sampling
			}
			ctx, cancel := context.WithTimeout(parent, time.Duration(m.currentConfig().Scanner.TimeoutMS+500)*time.Millisecond)
			delay, err := m.client.Delay(ctx, item.Name, item.Provider, probe, m.currentConfig().Scanner.TimeoutMS)
			cancel()
			entry := model.ProbeSample{Probe: probe.Name, DelayMS: delay}
			if err != nil {
				entry.DelayMS = 0
				entry.Error = err.Error()
			}
			result.Samples = append(result.Samples, entry)
		}
	}
	result.MeasuredAt = time.Now().UTC()
	result.ScreeningSamples = len(result.Samples)
	if mode == "refine" {
		result.Stage = "refined"
		result.ScreeningSamples = 0
		result.RefinementSamples = len(result.Samples)
	}
	calculateMetrics(&result, m.currentConfig().Scanner)
	assessResult(&result, profile, m.currentConfig().Scanner.StrictVerification.Enabled)
	return result
}
