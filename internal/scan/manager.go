package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strings"
	"sync"
	"time"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
	"github.com/yw-li/mihomo-smart-selector/internal/history"
	"github.com/yw-li/mihomo-smart-selector/internal/mihomo"
	"github.com/yw-li/mihomo-smart-selector/internal/model"
	"github.com/yw-li/mihomo-smart-selector/internal/regions"
)

type Event struct {
	Kind     string              `json:"kind"`
	Message  string              `json:"message"`
	At       time.Time           `json:"at"`
	Result   *model.NodeResult   `json:"result,omitempty"`
	Progress *model.ScanProgress `json:"progress,omitempty"`
}

type candidate struct {
	Name     string
	Provider string
	Region   regions.Match
}

type Manager struct {
	cfg        config.Config
	client     mihomo.Client
	classifier *regions.Classifier
	store      *history.Store

	mu          sync.Mutex
	active      map[string]*model.Scan
	subscribers map[string]map[chan Event]struct{}
}

func NewManager(cfg config.Config, client mihomo.Client, store *history.Store) *Manager {
	return &Manager{
		cfg: cfg, client: client, store: store, classifier: regions.New(cfg),
		active: map[string]*model.Scan{}, subscribers: map[string]map[chan Event]struct{}{},
	}
}

func (m *Manager) Start(request model.ScanRequest) (model.Scan, error) {
	request.TargetGroup = strings.TrimSpace(request.TargetGroup)
	if request.TargetGroup == "" {
		return model.Scan{}, fmt.Errorf("target_group is required")
	}
	if request.Mode == "" {
		request.Mode = "stable"
	}
	if request.Mode != "stable" && request.Mode != "quick" {
		return model.Scan{}, fmt.Errorf("mode must be stable or quick")
	}
	request.Regions = normaliseCodes(request.Regions)
	request.Providers = normaliseStrings(request.Providers)
	now := time.Now().UTC()
	scan := model.Scan{ID: newID(), Status: model.ScanRunning, Request: request, StartedAt: now}
	if err := m.store.CreateScan(context.Background(), scan); err != nil {
		return model.Scan{}, err
	}
	m.setActive(scan)
	m.publish(scan.ID, Event{Kind: "started", Message: "scan queued", At: now})
	go m.run(scan)
	return scan, nil
}

func (m *Manager) Get(ctx context.Context, id string) (model.Scan, error) {
	m.mu.Lock()
	if active, exists := m.active[id]; exists {
		snapshot := cloneScan(*active)
		m.mu.Unlock()
		return snapshot, nil
	}
	m.mu.Unlock()
	return m.store.GetScan(ctx, id)
}

func (m *Manager) Groups(ctx context.Context) ([]mihomo.Proxy, error) {
	proxies, err := m.client.ListProxies(ctx)
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
	providers, err := m.client.ListProviders(ctx)
	if err != nil {
		return nil, err
	}
	sort.Slice(providers, func(left, right int) bool { return providers[left].Name < providers[right].Name })
	return providers, nil
}

func (m *Manager) Regions() []config.Region {
	return append([]config.Region(nil), m.cfg.Regions...)
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

func (m *Manager) Select(ctx context.Context, scanID, requestedNode string) (model.SwitchEvent, error) {
	scan, err := m.store.GetScan(ctx, scanID)
	if err != nil {
		return model.SwitchEvent{}, err
	}
	if scan.Status != model.ScanComplete {
		return model.SwitchEvent{}, fmt.Errorf("scan %q is not complete", scanID)
	}
	var selected *model.NodeResult
	for index := range scan.Results {
		if requestedNode == "" && scan.Results[index].Rank == 1 {
			selected = &scan.Results[index]
			break
		}
		if requestedNode != "" && scan.Results[index].Name == requestedNode {
			selected = &scan.Results[index]
			break
		}
	}
	if selected == nil {
		return model.SwitchEvent{}, fmt.Errorf("requested node is not a result of scan %q", scanID)
	}
	if selected.SuccessRate < m.cfg.Scanner.MinSuccessRate {
		return model.SwitchEvent{}, fmt.Errorf("node %q is below min_success_rate", selected.Name)
	}
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return model.SwitchEvent{}, err
	}
	group, exists := proxies[scan.Request.TargetGroup]
	if !exists || !strings.EqualFold(group.Type, "Selector") {
		return model.SwitchEvent{}, fmt.Errorf("target group %q is no longer a selector", scan.Request.TargetGroup)
	}
	if !contains(group.All, selected.Name) {
		return model.SwitchEvent{}, fmt.Errorf("node %q is no longer a member of target group", selected.Name)
	}
	if err := m.client.Select(ctx, group.Name, selected.Name); err != nil {
		return model.SwitchEvent{}, err
	}
	event := model.SwitchEvent{
		ScanID: scan.ID, Group: group.Name, Previous: group.Now, Selected: selected.Name,
		Reason: "manual selection from completed scan", CreatedAt: time.Now().UTC(),
	}
	stored, err := m.store.RecordSwitch(ctx, event)
	if err == nil {
		m.publish(scan.ID, Event{Kind: "selected", Message: "selector changed", At: stored.CreatedAt})
	}
	return stored, err
}

func (m *Manager) Subscribe(scanID string) (<-chan Event, func()) {
	channel := make(chan Event, 16)
	m.mu.Lock()
	if m.subscribers[scanID] == nil {
		m.subscribers[scanID] = map[chan Event]struct{}{}
	}
	m.subscribers[scanID][channel] = struct{}{}
	m.mu.Unlock()
	return channel, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if subscribers := m.subscribers[scanID]; subscribers != nil {
			delete(subscribers, channel)
			if len(subscribers) == 0 {
				delete(m.subscribers, scanID)
			}
		}
		close(channel)
	}
}

func (m *Manager) run(scan model.Scan) {
	results, err := m.scan(context.Background(), scan)
	completed := time.Now().UTC()
	scan.Progress = m.progressSnapshot(scan.ID)
	if err != nil {
		scan.Status = model.ScanFailed
		scan.Error = err.Error()
	} else {
		scan.Status = model.ScanComplete
		scan.Results = results
	}
	scan.CompletedAt = &completed
	if persistErr := m.store.CompleteScan(context.Background(), scan); persistErr != nil {
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

func (m *Manager) scan(ctx context.Context, scan model.Scan) ([]model.NodeResult, error) {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover proxies: %w", err)
	}
	group, exists := proxies[scan.Request.TargetGroup]
	if !exists || !strings.EqualFold(group.Type, "Selector") {
		return nil, fmt.Errorf("target group %q is not a Mihomo Selector", scan.Request.TargetGroup)
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
	} else if len(scan.Request.Providers) > 0 {
		return nil, fmt.Errorf("discover providers for selected provider filter: %w", providerErr)
	}
	candidates := m.filterCandidates(group, proxies, providerByNode, scan.Request)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("no selector members matched the requested filters")
	}
	if len(candidates) > m.cfg.Scanner.MaxTotalCandidates {
		return nil, fmt.Errorf("%d candidates exceed scanner.max_total_candidates (%d); narrow regions or providers", len(candidates), m.cfg.Scanner.MaxTotalCandidates)
	}

	totalBatches := (len(candidates) + m.cfg.Scanner.BatchSize - 1) / m.cfg.Scanner.BatchSize
	m.setProgress(scan.ID, model.ScanProgress{Total: len(candidates), TotalBatches: totalBatches})
	allResults := make([]model.NodeResult, 0, len(candidates))
	for batchIndex, start := 1, 0; start < len(candidates); batchIndex, start = batchIndex+1, start+m.cfg.Scanner.BatchSize {
		end := min(start+m.cfg.Scanner.BatchSize, len(candidates))
		progress := model.ScanProgress{
			Completed: m.progressCompleted(scan.ID), Total: len(candidates),
			CurrentBatch: batchIndex, TotalBatches: totalBatches,
			BatchTotal: end - start,
		}
		m.setProgress(scan.ID, progress)
		m.publish(scan.ID, Event{Kind: "batch-started", Message: fmt.Sprintf("batch %d of %d started", batchIndex, totalBatches), At: time.Now().UTC(), Progress: &progress})
		allResults = append(allResults, m.probeBatch(ctx, scan, candidates[start:end])...)
	}
	if m.cfg.EgressVerification.Enabled {
		m.verifyEgress(ctx, proxies, allResults, scan.ID)
		for index := range allResults {
			calculateMetrics(&allResults[index], m.cfg.Scanner)
		}
	}
	sort.SliceStable(allResults, func(left, right int) bool {
		if allResults[left].Score != allResults[right].Score {
			return allResults[left].Score > allResults[right].Score
		}
		if allResults[left].SuccessRate != allResults[right].SuccessRate {
			return allResults[left].SuccessRate > allResults[right].SuccessRate
		}
		return allResults[left].P95MS < allResults[right].P95MS
	})
	for index := range allResults {
		allResults[index].Rank = index + 1
	}
	return allResults, nil
}

func (m *Manager) probeBatch(parent context.Context, scan model.Scan, candidates []candidate) []model.NodeResult {
	jobs := make(chan candidate)
	results := make(chan model.NodeResult, len(candidates))
	var workers sync.WaitGroup
	for worker := 0; worker < m.cfg.Scanner.Concurrency; worker++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			for item := range jobs {
				results <- m.probeCandidate(parent, item, scan.Request.Mode)
			}
		}()
	}
	go func() {
		for _, item := range candidates {
			jobs <- item
		}
		close(jobs)
		workers.Wait()
		close(results)
	}()

	batchResults := make([]model.NodeResult, 0, len(candidates))
	for result := range results {
		progress := m.recordCandidate(scan.ID, result)
		copy := result
		m.publish(scan.ID, Event{Kind: "candidate-complete", Message: "candidate tested: " + result.Name, At: time.Now().UTC(), Result: &copy, Progress: &progress})
		batchResults = append(batchResults, result)
	}
	return batchResults
}

func (m *Manager) filterCandidates(group mihomo.Proxy, proxies map[string]mihomo.Proxy, providerByNode map[string]string, request model.ScanRequest) []candidate {
	seen := map[string]bool{}
	items := make([]candidate, 0, len(group.All))
	for _, name := range group.All {
		if seen[name] {
			continue
		}
		seen[name] = true
		proxy, exists := proxies[name]
		if !exists {
			continue
		}
		if isPolicyGroup(proxy.Type) {
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

func (m *Manager) probeCandidate(parent context.Context, item candidate, mode string) model.NodeResult {
	result := model.NodeResult{
		Name: item.Name, Provider: item.Provider, InferredRegion: item.Region.Code, RegionSource: item.Region.Source,
	}
	samples := m.cfg.Scanner.Samples
	if mode == "quick" {
		samples = 1
	}
	for _, probe := range m.cfg.Scanner.Probes {
		for sample := 0; sample < samples; sample++ {
			ctx, cancel := context.WithTimeout(parent, time.Duration(m.cfg.Scanner.TimeoutMS+500)*time.Millisecond)
			delay, err := m.client.Delay(ctx, item.Name, item.Provider, probe, m.cfg.Scanner.TimeoutMS)
			cancel()
			entry := model.ProbeSample{Probe: probe.Name, DelayMS: delay}
			if err != nil {
				entry.DelayMS = 0
				entry.Error = err.Error()
			}
			result.Samples = append(result.Samples, entry)
		}
	}
	calculateMetrics(&result, m.cfg.Scanner)
	return result
}

func (m *Manager) verifyEgress(ctx context.Context, proxies map[string]mihomo.Proxy, results []model.NodeResult, scanID string) {
	probeSelector, exists := proxies[m.cfg.EgressVerification.SelectorGroup]
	if !exists || !strings.EqualFold(probeSelector.Type, "Selector") {
		for index := range results {
			results[index].EgressError = "configured probe selector is unavailable"
		}
		return
	}
	proxyURL, err := url.Parse(m.cfg.EgressVerification.ProxyURL)
	if err != nil {
		for index := range results {
			results[index].EgressError = "configured probe listener URL is invalid"
		}
		return
	}
	httpClient := &http.Client{
		Timeout: time.Duration(m.cfg.Scanner.TimeoutMS+1000) * time.Millisecond,
		Transport: &http.Transport{
			Proxy:       http.ProxyURL(proxyURL),
			DialContext: (&net.Dialer{Timeout: time.Duration(m.cfg.Scanner.TimeoutMS) * time.Millisecond}).DialContext,
		},
	}
	for index := range results {
		if !contains(probeSelector.All, results[index].Name) {
			results[index].EgressError = "candidate is not a member of configured probe selector"
			continue
		}
		if err := m.client.Select(ctx, probeSelector.Name, results[index].Name); err != nil {
			results[index].EgressError = "could not select candidate in probe selector"
			continue
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.cfg.EgressVerification.TraceURL, nil)
		if err != nil {
			results[index].EgressError = "could not create egress probe request"
			continue
		}
		response, err := httpClient.Do(request)
		if err != nil {
			results[index].EgressError = "egress trace request failed"
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK {
			results[index].EgressError = "egress trace response was not usable"
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			key, value, found := strings.Cut(line, "=")
			if found && key == "loc" && len(value) == 2 {
				results[index].VerifiedRegion = strings.ToUpper(value)
			}
		}
		if results[index].VerifiedRegion == "" {
			results[index].EgressError = "egress trace did not include a country code"
		}
		copy := results[index]
		m.publish(scanID, Event{Kind: "egress-verified", Message: "egress verification completed: " + results[index].Name, At: time.Now().UTC(), Result: &copy})
	}
	if probeSelector.Now != "" {
		_ = m.client.Select(ctx, probeSelector.Name, probeSelector.Now)
	}
}

func (m *Manager) publish(scanID string, event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for channel := range m.subscribers[scanID] {
		select {
		case channel <- event:
		default:
		}
	}
}

func (m *Manager) setActive(scan model.Scan) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := cloneScan(scan)
	m.active[scan.ID] = &snapshot
}

func (m *Manager) removeActive(scanID string) {
	m.mu.Lock()
	defer m.mu.Unlock()
	delete(m.active, scanID)
}

func (m *Manager) setProgress(scanID string, progress model.ScanProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		active.Progress = progress
	}
}

func (m *Manager) progressCompleted(scanID string) int {
	return m.progressSnapshot(scanID).Completed
}

func (m *Manager) progressSnapshot(scanID string) model.ScanProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		return active.Progress
	}
	return model.ScanProgress{}
}

func (m *Manager) recordCandidate(scanID string, result model.NodeResult) model.ScanProgress {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		active.Results = append(active.Results, result)
		active.Progress.Completed++
		active.Progress.BatchCompleted++
		return active.Progress
	}
	return model.ScanProgress{}
}

func cloneScan(input model.Scan) model.Scan {
	output := input
	output.Request.Regions = append([]string(nil), input.Request.Regions...)
	output.Request.Providers = append([]string(nil), input.Request.Providers...)
	output.Results = make([]model.NodeResult, len(input.Results))
	for index, result := range input.Results {
		output.Results[index] = result
		output.Results[index].Samples = append([]model.ProbeSample(nil), result.Samples...)
	}
	return output
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

func isPolicyGroup(proxyType string) bool {
	switch strings.ToLower(strings.ReplaceAll(proxyType, "-", "")) {
	case "selector", "urltest", "fallback", "loadbalance", "smart", "relay":
		return true
	default:
		return false
	}
}

func newID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("scan-%d", time.Now().UTC().UnixNano())
}
