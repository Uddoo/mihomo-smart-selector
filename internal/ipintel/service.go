package ipintel

import (
	"context"
	"sync"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Each provider has its own request gate, credentials, cache and account backoff.
type Service struct{ clients map[string]*Client }

func NewService() *Service {
	s := &Service{clients: map[string]*Client{}}
	for _, id := range []string{"ipqualityscore", "scamalytics", "ipapi_is", "abuseipdb"} {
		s.clients[id] = newProvider(id)
	}
	return s
}

func (s *Service) LookupAll(ctx context.Context, ip string, cfg config.IPIntelligenceConfig, reserve func() bool) []model.IPIntelligence {
	providers := cfg.Providers()
	results := make([]model.IPIntelligence, len(providers))
	var wg sync.WaitGroup
	for i, p := range providers {
		wg.Add(1)
		go func() {
			defer wg.Done()
			client := s.clients[p.ID]
			if p.Credentials.Enabled != nil && !*p.Credentials.Enabled {
				results[i] = client.empty("disabled")
				return
			}
			key, err := p.Credentials.APIKey()
			if err != nil {
				value := client.empty("error")
				value.Error = err.Error()
				results[i] = value
				return
			}
			results[i] = client.lookup(ctx, ip, credentials{key: key, username: p.Username, server: p.Server}, reserve)
		}()
	}
	wg.Wait()
	return results
}
