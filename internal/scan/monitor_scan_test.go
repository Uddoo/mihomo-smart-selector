package scan

import (
	"context"
	"errors"
	"path/filepath"
	"reflect"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type monitorScanController struct {
	*fakeMihomo
	providers   []mihomo.Provider
	providerErr error
}

func (c *monitorScanController) ListProviders(context.Context) ([]mihomo.Provider, error) {
	return c.providers, c.providerErr
}

func TestValidateMonitorScan(t *testing.T) {
	const (
		scopeError   = "扫描不属于当前 Controller，请重新扫描"
		statusError  = "扫描完成后可替换监控列表。"
		rankingError = "扫描排名与提交名单不一致，请重新读取扫描。"
		nodeError    = "排名中的节点已失效，请重新扫描后替换。"
	)
	type testCase struct {
		name   string
		change func(*model.Scan, *monitorScanController)
		scanID string
		nodes  []string
		want   string
	}
	tests := []testCase{
		{name: "ranked prefix includes failed expired unrefined and unverified results"},
		{name: "unknown scan", scanID: "absent", want: `scan "absent" not found`},
		{name: "single top node", nodes: []string{"a"}},
		{name: "all nodes", nodes: []string{"a", "c", "b"}},
		{name: "matching empty providers", change: func(s *model.Scan, c *monitorScanController) {
			for i := range s.Results {
				s.Results[i].Provider = ""
			}
			c.providers = nil
		}},
		{name: "unselected provider may change", change: func(_ *model.Scan, c *monitorScanController) {
			c.providers = []mihomo.Provider{
				{Name: "provider-a", Proxies: []mihomo.Proxy{{Name: "a"}, {Name: "c"}}},
				{Name: "provider-b", Proxies: []mihomo.Proxy{{Name: "b"}}},
			}
		}},
		{name: "foreign controller", want: scopeError, change: func(s *model.Scan, _ *monitorScanController) {
			s.ControllerScope = "http://previous-controller.example:9090"
		}},
		{name: "legacy empty scope", want: scopeError, change: func(s *model.Scan, _ *monitorScanController) {
			s.ControllerScope = ""
		}},
		{name: "different group", want: rankingError, change: func(s *model.Scan, _ *monitorScanController) {
			s.Request.TargetGroup = "other"
		}},
		{name: "no submitted nodes", nodes: []string{}, want: rankingError},
		{name: "too many nodes", nodes: []string{"a", "c", "b", "extra"}, want: rankingError},
		{name: "no stored results", want: rankingError, change: func(s *model.Scan, _ *monitorScanController) {
			s.Results = nil
		}},
		{name: "reordered prefix", nodes: []string{"c", "a"}, want: rankingError},
		{name: "skipped failed top node", nodes: []string{"c", "b"}, want: rankingError},
		{name: "noncontiguous prefix", nodes: []string{"a", "b"}, want: rankingError},
		{name: "duplicate node", nodes: []string{"a", "a"}, want: rankingError},
		{name: "unknown node", nodes: []string{"a", "missing"}, want: rankingError},
		{name: "provider replaced under same name", want: nodeError, change: func(_ *model.Scan, c *monitorScanController) {
			c.providers[0].Name = "provider-b"
		}},
		{name: "empty stored provider is not a wildcard", want: nodeError, change: func(s *model.Scan, _ *monitorScanController) {
			s.Results[0].Provider = ""
		}},
		{name: "empty current provider is not a wildcard", want: nodeError, change: func(_ *model.Scan, c *monitorScanController) {
			c.providers = nil
		}},
		{name: "second ranked provider changed", want: nodeError, change: func(s *model.Scan, _ *monitorScanController) {
			s.Results[2].Provider = "previous-provider"
		}},
		{name: "missing top member has no replacement", want: nodeError, change: func(_ *model.Scan, c *monitorScanController) {
			group := c.proxies["work"]
			group.All = []string{"b", "c"}
			c.proxies["work"] = group
		}},
		{name: "ambiguous provider", want: nodeError, change: func(_ *model.Scan, c *monitorScanController) {
			c.providers = append(c.providers, mihomo.Provider{Name: "provider-b", Proxies: []mihomo.Proxy{{Name: "a"}}})
		}},
		{name: "provider discovery fails closed", want: "无法核实 Provider 身份，暂停节点探测", change: func(_ *model.Scan, c *monitorScanController) {
			c.providerErr = errors.New("provider discovery unavailable")
		}},
	}
	for _, status := range []model.ScanStatus{model.ScanPending, model.ScanRunning, model.ScanInterrupted, model.ScanCancelled, model.ScanFailed} {
		tests = append(tests, testCase{
			name: string(status), want: statusError,
			change: func(s *model.Scan, _ *monitorScanController) { s.Status = status },
		})
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			ctx := context.Background()
			store, err := history.Open(filepath.Join(t.TempDir(), "scans.db"))
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			controller := &monitorScanController{
				fakeMihomo: &fakeMihomo{proxies: map[string]mihomo.Proxy{
					"work": {Name: "work", Type: "Selector", All: []string{"a", "b", "c"}},
					"a":    {Name: "a", Type: "VLESS"},
					"b":    {Name: "b", Type: "VLESS"},
					"c":    {Name: "c", Type: "VLESS"},
				}},
				providers: []mihomo.Provider{{Name: "provider-a", Proxies: []mihomo.Proxy{{Name: "a"}, {Name: "b"}, {Name: "c"}}}},
			}
			cfg := config.Defaults()
			for i := range cfg.Scanner.ProbeProfiles {
				cfg.Scanner.ProbeProfiles[i].RequireStrict = true
			}
			manager := NewManager(cfg, controller, store)
			old := time.Now().UTC().Add(-24 * time.Hour)
			stored := model.Scan{
				ID: "monitor-source", ControllerScope: manager.bindingScope(), Status: model.ScanComplete,
				Request:   model.ScanRequest{TargetGroup: "work", ProfileID: "chatgpt", Mode: "stable"},
				StartedAt: old, CompletedAt: &old,
				Results: []model.NodeResult{
					{Name: "a", Provider: "provider-a", Rank: 1, Stage: "refined", Score: 50, SuccessRate: 0, MeasuredAt: old, StrictVerificationStatus: "failed"},
					{Name: "b", Provider: "provider-a", Rank: 2, Stage: "screened", Score: 90, SuccessRate: 1, MeasuredAt: old},
					{Name: "c", Provider: "provider-a", Rank: 3, Stage: "refined", Score: 40, SuccessRate: 1, MeasuredAt: old},
				},
			}
			if tt.change != nil {
				tt.change(&stored, controller)
			}
			if err := store.CreateScan(ctx, stored); err != nil {
				t.Fatal(err)
			}
			if err := store.CompleteScan(ctx, stored); err != nil {
				t.Fatal(err)
			}
			before, err := store.GetScan(ctx, stored.ID)
			if err != nil {
				t.Fatal(err)
			}
			// An active snapshot must not override the persisted source scan.
			manager.active[stored.ID] = &model.Scan{ID: stored.ID, Status: model.ScanRunning}
			nodes := tt.nodes
			if nodes == nil {
				nodes = []string{"a", "c"}
			}
			scanID := tt.scanID
			if scanID == "" {
				scanID = stored.ID
			}
			err = manager.ValidateMonitorScan(ctx, scanID, "work", "chatgpt", nodes)
			if tt.want == "" {
				if err != nil {
					t.Fatal(err)
				}
			} else if err == nil || err.Error() != tt.want {
				t.Fatalf("validation error = %v, want %q", err, tt.want)
			}
			after, err := store.GetScan(ctx, stored.ID)
			if err != nil || !reflect.DeepEqual(before, after) {
				t.Fatalf("validation mutated stored scan: %v", err)
			}
		})
	}
}
