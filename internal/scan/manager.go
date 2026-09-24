package scan

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/regions"
	"sync"
	"sync/atomic"
	"time"
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
	resultIndex    map[string]map[string]int
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
	scan := model.Scan{ID: newID(), ControllerScope: m.bindingScope(), Status: model.ScanRunning, Request: request, Profile: m.profileSummary(profile), StartedAt: now}
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

func newID() string {
	bytes := make([]byte, 12)
	if _, err := rand.Read(bytes); err == nil {
		return hex.EncodeToString(bytes)
	}
	return fmt.Sprintf("scan-%d", time.Now().UTC().UnixNano())
}
