package scan

import (
	"context"
	"testing"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestDiscoveryPreservesControllerSnapshot(t *testing.T) {
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{
		"JP-01":   {Name: "JP-01", Type: "Vless"},
		"ChatGPT": {Name: "ChatGPT", Type: "Selector", All: []string{"JP-01"}},
	}}
	manager := NewManager(config.Defaults(), fake, nil)
	candidates, err := manager.discoverCandidates(context.Background(), model.ScanRequest{TargetGroup: "ChatGPT"})
	if err != nil || len(candidates) != 1 {
		t.Fatalf("discovery: candidates=%v, err=%v", candidates, err)
	}
	// fakeMihomo also returns provider-only JP-03. Discovery must not insert it
	// into a snapshot that another concurrent scan can be reading.
	if _, exists := fake.proxies["JP-03"]; exists || len(fake.proxies) != 2 {
		t.Fatalf("discovery mutated the controller snapshot: %+v", fake.proxies)
	}
}
