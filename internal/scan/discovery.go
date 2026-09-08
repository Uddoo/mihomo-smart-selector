package scan

import (
	"context"
	"sync"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
)

// Only read-only catalog endpoints use this cache. Scans and selection always
// discover fresh membership through the underlying client.
type discoveryCache[T any] struct {
	mu       sync.Mutex
	value    T
	until    time.Time
	flight   *discoveryFlight[T]
	revision uint64
}

type discoveryFlight[T any] struct {
	done  chan struct{}
	value T
	err   error
}

func (c *discoveryCache[T]) get(ctx context.Context, fetch func(context.Context) (T, error), clone func(T) T) (T, error) {
	for {
		c.mu.Lock()
		if time.Now().Before(c.until) {
			value := clone(c.value)
			c.mu.Unlock()
			return value, nil
		}
		if c.flight != nil {
			flight := c.flight
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				var zero T
				return zero, ctx.Err()
			case <-flight.done:
				return clone(flight.value), flight.err
			}
		}
		c.flight = &discoveryFlight[T]{done: make(chan struct{})}
		revision := c.revision
		c.mu.Unlock()
		value, err := fetch(ctx)
		c.mu.Lock()
		if err == nil && revision == c.revision {
			c.value = clone(value)
			c.until = time.Now().Add(2 * time.Second)
		}
		c.flight.value = clone(value)
		c.flight.err = err
		close(c.flight.done)
		c.flight = nil
		c.mu.Unlock()
		return value, err
	}
}

func cloneProxies(input map[string]mihomo.Proxy) map[string]mihomo.Proxy {
	out := make(map[string]mihomo.Proxy, len(input))
	for key, p := range input {
		p.All = append([]string(nil), p.All...)
		out[key] = p
	}
	return out
}
func cloneProviders(input []mihomo.Provider) []mihomo.Provider {
	out := append([]mihomo.Provider(nil), input...)
	for i := range out {
		out[i].Proxies = append([]mihomo.Proxy(nil), out[i].Proxies...)
		for j := range out[i].Proxies {
			out[i].Proxies[j].All = append([]string(nil), out[i].Proxies[j].All...)
		}
	}
	return out
}
func (m *Manager) catalogProxies(ctx context.Context) (map[string]mihomo.Proxy, error) {
	return m.proxyCache.get(ctx, m.client.ListProxies, cloneProxies)
}
func (m *Manager) catalogProviders(ctx context.Context) ([]mihomo.Provider, error) {
	return m.providerCache.get(ctx, m.client.ListProviders, cloneProviders)
}
func (m *Manager) invalidateCatalog() {
	m.proxyCache.mu.Lock()
	m.proxyCache.until = time.Time{}
	m.proxyCache.revision++
	m.proxyCache.mu.Unlock()
}
