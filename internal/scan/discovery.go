package scan

import (
	"context"
	"sync"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
)

// Catalog endpoints and preflight share this cache. Scans and selection always
// discover fresh membership through the underlying client.
type discoveryCache[T any] struct {
	mu       sync.Mutex
	value    T
	until    time.Time
	flight   *discoveryFlight[T]
	revision uint64
}

type discoveryFlight[T any] struct {
	done    chan struct{}
	cancel  context.CancelFunc
	waiters int
	value   T
	err     error
}

func (c *discoveryCache[T]) get(ctx context.Context, fetch func(context.Context) (T, error), clone func(T) T) (T, error) {
	var zero T
	c.mu.Lock()
	if err := ctx.Err(); err != nil {
		c.mu.Unlock()
		return zero, err
	}
	if time.Now().Before(c.until) {
		value := c.value
		c.mu.Unlock()
		return clone(value), nil
	}
	flight := c.flight
	if flight == nil {
		// Every caller can leave independently. The last one cancels the read.
		readCtx, cancel := context.WithCancel(context.WithoutCancel(ctx))
		flight = &discoveryFlight[T]{done: make(chan struct{}), cancel: cancel}
		c.flight = flight
		go c.runFlight(readCtx, flight, c.revision, fetch, clone)
	}
	flight.waiters++
	c.mu.Unlock()

	select {
	case <-ctx.Done():
		c.mu.Lock()
		flight.waiters--
		if flight.waiters == 0 && c.flight == flight {
			c.flight = nil
			flight.cancel()
		}
		c.mu.Unlock()
	case <-flight.done:
	}
	if err := ctx.Err(); err != nil {
		return zero, err
	}
	return clone(flight.value), flight.err
}

func (c *discoveryCache[T]) runFlight(ctx context.Context, flight *discoveryFlight[T], revision uint64, fetch func(context.Context) (T, error), clone func(T) T) {
	defer flight.cancel()
	value, err := fetch(ctx)
	flight.value, flight.err = clone(value), err

	c.mu.Lock()
	defer c.mu.Unlock()
	// An abandoned read can finish after a replacement has already started.
	if c.flight == flight {
		if err == nil && revision == c.revision {
			c.value = flight.value
			c.until = time.Now().Add(2 * time.Second)
		}
		c.flight = nil
	}
	close(flight.done)
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
