package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"maps"
	"sort"
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
	probeSlots        chan struct{}
	workers           sync.WaitGroup
	stopping          bool
	maintenanceCancel context.CancelFunc
	proxyCache        discoveryCache[map[string]mihomo.Proxy]
	providerCache     discoveryCache[[]mihomo.Provider]
	cfg               atomic.Pointer[config.Config]
	settingsMu        sync.Mutex
	settingsRevision  int
	client            mihomo.Client
	classifier        *regions.Classifier
	store             *history.Store

	mu             sync.Mutex
	active         map[string]*model.Scan
	cancels        map[string]context.CancelFunc
	stopAfterBatch map[string]bool
	subscribers    map[string]map[chan Event]struct{}
}

func NewManager(cfg config.Config, client mihomo.Client, store *history.Store) *Manager {
	m := &Manager{
		probeSlots: make(chan struct{}, cfg.Scanner.Concurrency),
		client:     client, store: store, classifier: regions.New(cfg),
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
	if m.stopping {
		return model.Scan{}, fmt.Errorf("服务正在停止")
	}
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
	if err := m.admitScan(request.TargetGroup); err != nil {
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
	m.workers.Add(1)
	go func() { defer m.workers.Done(); defer cancel(); m.run(ctx, scan) }()
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
		return m.decorateScan(snapshot), nil
	}
	m.mu.Unlock()
	stored, err := m.store.GetScan(ctx, id)
	if err != nil {
		return stored, err
	}
	return m.decorateScan(stored), nil
}

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
		RequireStrict: profile.RequireStrict, RequireRegion: profile.RequireRegion,
		ID: profile.ID, Label: profile.Label, Description: profile.Description,
		ProbeCount: len(profile.Probes), StrictProbeCount: len(profile.StrictProbes),
		StrictVerificationAvailable: len(profile.StrictProbes) > 0 && m.currentConfig().Scanner.StrictVerification.Enabled,
		RequiresConfiguration:       profile.RequiresConfiguration, SetupHint: profile.SetupHint,
		ExpectedRegions: append([]string(nil), profile.ExpectedRegions...), TransportScope: profile.TransportScope,
		Targets: targets,
	}
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

	allResults, err := m.stagedProbes(ctx, scan, profile, candidates, proxies[scan.Request.TargetGroup].Now)
	if err != nil {
		return allResults, err
	}
	rankNodes(allResults)

	if m.currentConfig().EgressVerification.Enabled {
		m.verifyEgress(ctx, proxies, allResults, scan.ID)
		for index := range allResults {
			calculateMetrics(&allResults[index], m.currentConfig().Scanner)
		}
	}
	m.verifyStrict(ctx, proxies, profile, allResults, scan.ID)
	if err := ctx.Err(); err != nil {
		return allResults, err
	}
	for index := range allResults {
		assessResult(&allResults[index], profile, m.currentConfig().Scanner.StrictVerification.Enabled)
	}
	rankNodes(allResults)
	return allResults, nil
}

func (m *Manager) discoverCandidates(ctx context.Context, request model.ScanRequest) ([]candidate, error) {
	proxies, err := m.client.ListProxies(ctx)
	if err != nil {
		return nil, fmt.Errorf("discover proxies: %w", err)
	}
	// Provider enrichment belongs to this scan, not the controller's shared snapshot.
	proxies = maps.Clone(proxies)
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
	request.Nodes = normaliseStrings(request.Nodes)
	request.Regions = normaliseCodes(request.Regions)
	request.Providers = normaliseStrings(request.Providers)
	return request, nil
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

func (m *Manager) recordCandidate(scanID string, result model.NodeResult, profile config.ProbeProfile) (model.NodeResult, model.ScanProgress) {
	m.mu.Lock()
	defer m.mu.Unlock()
	if active, exists := m.active[scanID]; exists {
		replaced := false
		for i := range active.Results {
			if active.Results[i].Name == result.Name {
				result.ScreeningSamples = active.Results[i].ScreeningSamples
				result.Samples = append(append([]model.ProbeSample(nil), active.Results[i].Samples...), result.Samples...)
				calculateMetrics(&result, m.currentConfig().Scanner)
				assessResult(&result, profile, m.currentConfig().Scanner.StrictVerification.Enabled)
				active.Results[i] = result
				replaced = true
				break
			}
		}
		if !replaced {
			active.Results = append(active.Results, result)
		}
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
		return result, active.Progress
	}
	return result, model.ScanProgress{}
}

func (m *Manager) stopRequested(scanID string) bool {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.stopAfterBatch[scanID]
}

func cloneScan(input model.Scan) model.Scan {
	output := input
	output.Request.Nodes = append([]string(nil), input.Request.Nodes...)
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
