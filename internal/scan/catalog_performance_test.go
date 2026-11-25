package scan

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
)

// Measure the work done by the nodes endpoint once controller discovery is cached.
func BenchmarkCachedNodeCatalog(b *testing.B) {
	labels := []struct{ text, region, source string }{
		{"🇭🇰 香港 HK", "HK", "name-inferred"},
		{"London", "GB", "name-inferred"},
		{"ＳＧ", "SG", "name-inferred"},
		{"US via JP", "", "ambiguous"},
		{"unlabelled", "", "unknown"},
		{"印度尼西亚", "ID", "name-inferred"},
		{"Tokyo", "JP", "name-inferred"},
		{"São Paulo", "BR", "name-inferred"},
	}
	proxies := make(map[string]mihomo.Proxy)
	want := make(map[string]int)
	provider := mihomo.Provider{Name: "fixture"}
	for i := 0; i < 256; i++ {
		label := i % len(labels)
		name := fmt.Sprintf("%s-%03d", labels[label].text, i)
		want[name] = label
		proxy := mihomo.Proxy{Name: name, Type: "VLESS"}
		provider.Proxies = append(provider.Proxies, proxy)
		proxies[name] = proxy
	}
	m := NewManager(config.Defaults(), &fakeMihomo{}, nil)
	until := time.Now().Add(24 * time.Hour)
	m.proxyCache.value, m.proxyCache.until = proxies, until
	m.providerCache.value, m.providerCache.until = []mihomo.Provider{provider}, until
	ctx := context.Background()
	nodes, err := m.Nodes(ctx)
	if err != nil || len(nodes) != len(want) {
		b.Fatalf("catalog size %d, error %v", len(nodes), err)
	}
	for _, node := range nodes {
		i, ok := want[node.Name]
		if !ok || node.InferredRegion != labels[i].region || node.RegionSource != labels[i].source {
			b.Fatalf("unexpected classification: %+v", node)
		}
		delete(want, node.Name)
	}
	b.ReportAllocs()
	for b.Loop() {
		nodes, err := m.Nodes(ctx)
		if err != nil || len(nodes) != len(proxies) {
			b.Fatalf("catalog size %d, error %v", len(nodes), err)
		}
	}
}
