package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"sort"
	"strconv"
	"strings"
	"sync"
	"sync/atomic"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/regions"
)

var (
	errScanStopped  = fmt.Errorf("scan stopped by operator")
	errNoCandidates = fmt.Errorf("no eligible leaf candidates")
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
	cfg              atomic.Pointer[config.Config]
	settingsMu       sync.Mutex
	settingsRevision int
	client           mihomo.Client
	classifier       *regions.Classifier
	store            *history.Store

	mu             sync.Mutex
	active         map[string]*model.Scan
	cancels        map[string]context.CancelFunc
	stopAfterBatch map[string]bool
	subscribers    map[string]map[chan Event]struct{}
}

func NewManager(cfg config.Config, client mihomo.Client, store *history.Store) *Manager {
	m := &Manager{
		client: client, store: store, classifier: regions.New(cfg),
		active: map[string]*model.Scan{}, cancels: map[string]context.CancelFunc{}, stopAfterBatch: map[string]bool{}, subscribers: map[string]map[chan Event]struct{}{},
	}
	m.cfg.Store(&cfg)
	return m
}

func (m *Manager) currentConfig() config.Config { return *m.cfg.Load() }

func (m *Manager) Start(request model.ScanRequest) (model.Scan, error) {
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
	if err := m.checkProbeTarget(request.TargetGroup); err != nil {
		return model.Scan{}, err
	}
	cfg := m.currentConfig()
	if (cfg.EgressVerification.Enabled || cfg.Scanner.StrictVerification.Enabled) && m.hasRunningScan() {
		return model.Scan{}, fmt.Errorf("验证使用共享探测组，请等待当前扫描结束")
	}
	var err error
	request, err = normaliseRequest(request)
	if err != nil {
		return model.Scan{}, err
	}
	profile, err := m.profileFor(request)
	if err != nil {
		return model.Scan{}, err
	}
	// Freeze the resolved service so later binding edits cannot change this scan.
	request.ProfileID = profile.ID
	now := time.Now().UTC()
	scan := model.Scan{ID: newID(), Status: model.ScanRunning, Request: request, Profile: m.profileSummary(profile), StartedAt: now}
	if err := m.store.CreateScan(context.Background(), scan); err != nil {
		return model.Scan{}, err
	}
	ctx, cancel := context.WithCancel(context.Background())
	m.setActiveWithCancel(scan, cancel)
	m.publish(scan.ID, Event{Kind: "started", Message: "scan queued", At: now})
	go m.run(ctx, scan)
	return scan, nil
}

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
	candidates, err := m.discoverCandidates(ctx, request)
	if err != nil {
		if errors.Is(err, errNoCandidates) {
			return model.ScanPreview{
				Profile: m.profileSummary(profile), Ready: false,
				Reason: "此选择器当前未直接包含可选择的叶子节点；为避免越过嵌套策略组，扫描已保持禁用。",
			}, nil
		}
		return model.ScanPreview{}, err
	}
	if len(candidates) > m.currentConfig().Scanner.MaxTotalCandidates {
		return model.ScanPreview{}, fmt.Errorf("%d candidates exceed scanner.max_total_candidates (%d); narrow regions or providers", len(candidates), m.currentConfig().Scanner.MaxTotalCandidates)
	}
	samples := m.currentConfig().Scanner.Samples
	if request.Mode == "quick" {
		samples = 1
	}
	return model.ScanPreview{
		CandidateCount: len(candidates), BatchSize: m.currentConfig().Scanner.BatchSize,
		BatchCount:     (len(candidates) + m.currentConfig().Scanner.BatchSize - 1) / m.currentConfig().Scanner.BatchSize,
		SamplesPerNode: samples, ProbeRequests: len(candidates) * len(profile.Probes) * samples,
		Profile: m.profileSummary(profile), Ready: true,
	}, nil
}

func (m *Manager) Stop(scanID string, afterCurrentBatch bool) (model.ScanProgress, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	active, exists := m.active[scanID]
	if !exists || active.Status != model.ScanRunning {
		return model.ScanProgress{}, fmt.Errorf("scan %q is not running", scanID)
	}
	if afterCurrentBatch {
		m.stopAfterBatch[scanID] = true
		active.Progress.StopAfterCurrentBatch = true
		return active.Progress, nil
	}
	if cancel := m.cancels[scanID]; cancel != nil {
		cancel()
	}
	return active.Progress, nil
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
	return append([]config.Region(nil), m.currentConfig().Regions...)
}

func (m *Manager) profileFor(request model.ScanRequest) (config.ProbeProfile, error) {
	profile, err := m.resolveService(context.Background(), request)
	if err != nil {
		return config.ProbeProfile{}, err
	}
	if profile.RequiresConfiguration {
		return config.ProbeProfile{}, fmt.Errorf("probe profile %q needs configuration: %s", profile.Label, profile.SetupHint)
	}
	if len(profile.Probes) == 0 {
		return config.ProbeProfile{}, fmt.Errorf("probe profile %q has no reachable probes", profile.Label)
	}
	return profile, nil
}

func (m *Manager) profileSummary(profile config.ProbeProfile) model.ProbeProfileSummary {
	targets := make([]model.ProbeTargetSummary, 0, len(profile.Probes)+len(profile.StrictProbes))
	for _, probe := range profile.Probes {
		targets = append(targets, model.ProbeTargetSummary{
			Name: probe.Name, ExpectedStatus: probe.ExpectedStatus, Kind: "reachability", AddressVisible: profile.ExposeTargetAddresses,
		})
		if profile.ExposeTargetAddresses {
			targets[len(targets)-1].Address = probe.URL
		}
	}
	for _, probe := range profile.StrictProbes {
		targets = append(targets, model.ProbeTargetSummary{
			Name: probe.Name, ExpectedStatus: probe.ExpectedStatus, Kind: "strict", AddressVisible: profile.ExposeTargetAddresses,
		})
		if profile.ExposeTargetAddresses {
			targets[len(targets)-1].Address = probe.URL
		}
	}
	return model.ProbeProfileSummary{
		ID: profile.ID, Label: profile.Label, Description: profile.Description,
		ProbeCount: len(profile.Probes), StrictProbeCount: len(profile.StrictProbes),
		StrictVerificationAvailable: len(profile.StrictProbes) > 0 && m.currentConfig().Scanner.StrictVerification.Enabled,
		RequiresConfiguration:       profile.RequiresConfiguration, SetupHint: profile.SetupHint,
		ExpectedRegions: append([]string(nil), profile.ExpectedRegions...), TransportScope: profile.TransportScope,
		Targets: targets,
	}
}

func (m *Manager) Nodes(ctx context.Context) ([]model.NodeSummary, error) {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return nil, err
	}
	providers, err := m.client.ListProviders(ctx)
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
			match := m.classifier.Classify(proxy.Name)
			nodes = append(nodes, model.NodeSummary{Name: proxy.Name, Provider: provider.Name, Protocol: proxy.Type, InferredRegion: match.Code, RegionSource: match.Source})
		}
	}
	for name, proxy := range proxies {
		if name == "" || seen[name] || isPolicyGroup(proxy.Type) {
			continue
		}
		seen[name] = true
		match := m.classifier.Classify(name)
		nodes = append(nodes, model.NodeSummary{Name: name, Provider: proxy.ProviderName, Protocol: proxy.Type, InferredRegion: match.Code, RegionSource: match.Source})
	}
	sort.Slice(nodes, func(left, right int) bool { return nodes[left].Name < nodes[right].Name })
	return nodes, nil
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
	m.settingsMu.Lock()
	defer m.settingsMu.Unlock()
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
	if selected.SuccessRate < m.currentConfig().Scanner.MinSuccessRate {
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

func (m *Manager) run(ctx context.Context, scan model.Scan) {
	results, err := m.scan(ctx, scan)
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

func (m *Manager) scan(ctx context.Context, scan model.Scan) ([]model.NodeResult, error) {
	profile, err := m.profileFor(scan.Request)
	if err != nil {
		return nil, err
	}
	candidates, err := m.discoverCandidates(ctx, scan.Request)
	if err != nil {
		return nil, err
	}
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover proxies for egress verification: %w", err)
	}
	if len(candidates) > m.currentConfig().Scanner.MaxTotalCandidates {
		return nil, fmt.Errorf("%d candidates exceed scanner.max_total_candidates (%d); narrow regions or providers", len(candidates), m.currentConfig().Scanner.MaxTotalCandidates)
	}

	totalBatches := (len(candidates) + m.currentConfig().Scanner.BatchSize - 1) / m.currentConfig().Scanner.BatchSize
	m.setProgress(scan.ID, model.ScanProgress{Total: len(candidates), TotalBatches: totalBatches})
	allResults := make([]model.NodeResult, 0, len(candidates))
	for batchIndex, start := 1, 0; start < len(candidates); batchIndex, start = batchIndex+1, start+m.currentConfig().Scanner.BatchSize {
		end := min(start+m.currentConfig().Scanner.BatchSize, len(candidates))
		progress := m.progressSnapshot(scan.ID)
		progress.Total = len(candidates)
		progress.CurrentBatch = batchIndex
		progress.TotalBatches = totalBatches
		progress.BatchCompleted = 0
		progress.BatchTotal = end - start
		m.setProgress(scan.ID, progress)
		m.publish(scan.ID, Event{Kind: "batch-started", Message: fmt.Sprintf("batch %d of %d started", batchIndex, totalBatches), At: time.Now().UTC(), Progress: &progress})
		allResults = append(allResults, m.probeBatch(ctx, scan, profile, candidates[start:end])...)
		if ctx.Err() != nil {
			return allResults, ctx.Err()
		}
		if m.stopRequested(scan.ID) {
			return allResults, errScanStopped
		}
	}

	if m.currentConfig().EgressVerification.Enabled {
		m.verifyEgress(ctx, proxies, allResults, scan.ID)
		for index := range allResults {
			calculateMetrics(&allResults[index], m.currentConfig().Scanner)
		}
	}
	m.verifyStrict(ctx, proxies, profile, allResults, scan.ID)
	for index := range allResults {
		assessResult(&allResults[index], profile, m.currentConfig().Scanner.StrictVerification.Enabled)
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

func (m *Manager) discoverCandidates(ctx context.Context, request model.ScanRequest) ([]candidate, error) {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover proxies: %w", err)
	}
	group, exists := proxies[request.TargetGroup]
	if !exists || !strings.EqualFold(group.Type, "Selector") {
		return nil, fmt.Errorf("target group %q is not a Mihomo Selector", request.TargetGroup)
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
		return nil, fmt.Errorf("discover providers for selected provider filter: %w", providerErr)
	}
	candidates := m.filterCandidates(group, proxies, providerByNode, request)
	if len(candidates) == 0 {
		return nil, fmt.Errorf("%w matched the requested filters", errNoCandidates)
	}
	return candidates, nil
}

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
	request.Regions = normaliseCodes(request.Regions)
	request.Providers = normaliseStrings(request.Providers)
	return request, nil
}

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
		for _, item := range candidates {
			select {
			case <-parent.Done():
				close(jobs)
				return
			case jobs <- item:
			}
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

func (m *Manager) probeCandidate(parent context.Context, item candidate, profile config.ProbeProfile, mode string) model.NodeResult {
	result := model.NodeResult{
		Name: item.Name, Provider: item.Provider, InferredRegion: item.Region.Code, RegionSource: item.Region.Source,
	}
	samples := m.currentConfig().Scanner.Samples
	if mode == "quick" {
		samples = 1
	}
	for _, probe := range profile.Probes {
		for sample := 0; sample < samples; sample++ {
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
	calculateMetrics(&result, m.currentConfig().Scanner)
	assessResult(&result, profile, m.currentConfig().Scanner.StrictVerification.Enabled)
	return result
}

func (m *Manager) verifyEgress(ctx context.Context, proxies map[string]mihomo.Proxy, results []model.NodeResult, scanID string) {
	probeSelector, exists := proxies[m.currentConfig().EgressVerification.SelectorGroup]
	if !exists || !strings.EqualFold(probeSelector.Type, "Selector") {
		for index := range results {
			results[index].EgressError = "configured probe selector is unavailable"
		}
		return
	}
	defer m.restoreProbeSelector(probeSelector.Name, probeSelector.Now)
	proxyURL, err := url.Parse(m.currentConfig().EgressVerification.ProxyURL)
	if err != nil {
		for index := range results {
			results[index].EgressError = "configured probe listener URL is invalid"
		}
		return
	}
	httpClient := &http.Client{
		Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS+1000) * time.Millisecond,
		Transport: &http.Transport{
			Proxy:       http.ProxyURL(proxyURL),
			DialContext: (&net.Dialer{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS) * time.Millisecond}).DialContext,
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
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.currentConfig().EgressVerification.TraceURL, nil)
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

// verifyStrict performs status-code and optional body checks through a
// dedicated, user-configured selector. The target business selector is never
// changed. Disabling this feature is the default because a router must first
// provide an isolated probe selector and local proxy listener.
func (m *Manager) verifyStrict(ctx context.Context, proxies map[string]mihomo.Proxy, profile config.ProbeProfile, results []model.NodeResult, scanID string) {
	if len(profile.StrictProbes) == 0 {
		return
	}
	verification := m.currentConfig().Scanner.StrictVerification
	if !verification.Enabled {
		for index := range results {
			results[index].StrictVerificationStatus = "not_configured"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	if len(results) > verification.MaxCandidates {
		for index := range results {
			results[index].StrictVerificationStatus = "not_run_limit"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	probeSelector, exists := proxies[verification.SelectorGroup]
	if !exists || !strings.EqualFold(probeSelector.Type, "Selector") {
		for index := range results {
			results[index].StrictVerificationStatus = "probe_selector_unavailable"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	proxyURL, err := url.Parse(verification.ProxyURL)
	if err != nil {
		for index := range results {
			results[index].StrictVerificationStatus = "probe_proxy_invalid"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true,
		DialContext:       (&net.Dialer{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS) * time.Millisecond}).DialContext,
	}
	client := &http.Client{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS+1000) * time.Millisecond, Transport: transport}
	defer transport.CloseIdleConnections()
	if probeSelector.Now != "" {
		defer m.restoreProbeSelector(probeSelector.Name, probeSelector.Now)
	}
	for index := range results {
		result := &results[index]
		if !contains(probeSelector.All, result.Name) {
			result.StrictVerificationStatus = "candidate_not_in_probe_selector"
			result.RestrictionStatus = "not_checked"
			continue
		}
		if err := m.client.Select(ctx, probeSelector.Name, result.Name); err != nil {
			result.StrictVerificationStatus = "probe_selector_switch_failed"
			result.RestrictionStatus = "not_checked"
			continue
		}
		for _, probe := range profile.StrictProbes {
			result.StrictChecks = append(result.StrictChecks, m.runStrictCheck(ctx, client, probe))
		}
		applyStrictOutcome(result)
		copy := *result
		m.publish(scanID, Event{Kind: "strict-verified", Message: "strict service verification completed: " + result.Name, At: time.Now().UTC(), Result: &copy})
	}
}

func (m *Manager) runStrictCheck(parent context.Context, client *http.Client, probe config.StrictProbe) model.StrictCheck {
	check := model.StrictCheck{Probe: probe.Name, ExpectedStatus: probe.ExpectedStatus, Status: "failed"}
	ctx, cancel := context.WithTimeout(parent, time.Duration(m.currentConfig().Scanner.TimeoutMS+500)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.URL, nil)
	if err != nil {
		check.Error = "could not create strict probe request"
		return check
	}
	response, err := client.Do(request)
	if err != nil {
		check.Error = "strict probe request failed"
		return check
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	check.ObservedStatus = response.StatusCode
	if readErr != nil {
		check.Error = "could not read strict probe response"
		return check
	}
	expected, _ := strconv.Atoi(probe.ExpectedStatus)
	if containsStatus(probe.RestrictedStatusCodes, response.StatusCode) {
		check.Status = "restricted"
		return check
	}
	if response.StatusCode != expected {
		check.Error = fmt.Sprintf("expected HTTP %d, received %d", expected, response.StatusCode)
		return check
	}
	if probe.BodyContains != "" {
		matched := strings.Contains(string(body), probe.BodyContains)
		check.BodyMatched = &matched
		if !matched {
			check.Error = "expected response content was not present"
			return check
		}
	}
	check.Status = "passed"
	return check
}

func (m *Manager) restoreProbeSelector(groupName, previous string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = m.client.Select(ctx, groupName, previous)
}

func applyStrictOutcome(result *model.NodeResult) {
	if len(result.StrictChecks) == 0 {
		return
	}
	passed := true
	for _, check := range result.StrictChecks {
		if check.Status == "restricted" {
			result.StrictVerificationStatus = "restricted"
			result.RestrictionStatus = "restricted"
			return
		}
		if check.Status != "passed" {
			passed = false
		}
	}
	if passed {
		result.StrictVerificationStatus = "passed"
		result.RestrictionStatus = "not_restricted"
		return
	}
	result.StrictVerificationStatus = "failed"
	result.RestrictionStatus = "unknown"
}

func assessResult(result *model.NodeResult, profile config.ProbeProfile, strictEnabled bool) {
	switch {
	case result.SuccessRate >= 1:
		result.ReachabilityStatus = "available"
	case result.SuccessRate > 0:
		result.ReachabilityStatus = "partial"
	default:
		result.ReachabilityStatus = "unavailable"
	}
	if len(profile.StrictProbes) == 0 {
		result.StrictVerificationStatus = "not_requested"
		result.RestrictionStatus = "not_checked"
	} else if !strictEnabled && result.StrictVerificationStatus == "" {
		result.StrictVerificationStatus = "not_configured"
		result.RestrictionStatus = "not_checked"
	}
	if len(profile.ExpectedRegions) == 0 {
		result.RegionVerificationStatus = "not_configured"
	} else if result.VerifiedRegion == "" {
		result.RegionVerificationStatus = "unverified"
	} else if containsFold(profile.ExpectedRegions, result.VerifiedRegion) {
		result.RegionVerificationStatus = "matched"
	} else {
		result.RegionVerificationStatus = "mismatch"
	}
	if profile.TransportScope == "" {
		result.TransportStatus = "latency_only"
	} else {
		result.TransportStatus = profile.TransportScope
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
	delete(m.cancels, scanID)
	delete(m.stopAfterBatch, scanID)
}

func (m *Manager) setActiveWithCancel(scan model.Scan, cancel context.CancelFunc) {
	m.mu.Lock()
	defer m.mu.Unlock()
	snapshot := cloneScan(scan)
	m.active[scan.ID] = &snapshot
	m.cancels[scan.ID] = cancel
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
		if result.SuccessRate > 0 {
			active.Progress.Succeeded++
		} else {
			active.Progress.Failed++
		}
		active.Progress.ElapsedSeconds = int(time.Since(active.StartedAt).Seconds())
		if active.Progress.Completed > 0 {
			remaining := active.Progress.Total - active.Progress.Completed
			active.Progress.EstimatedRemainingSeconds = active.Progress.ElapsedSeconds * remaining / active.Progress.Completed
		}
		return active.Progress
	}
	return model.ScanProgress{}
}

func (m *Manager) stopRequested(scanID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopAfterBatch[scanID]
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

func newID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("scan-%d", time.Now().UTC().UnixNano())
}
