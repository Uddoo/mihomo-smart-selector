package scan

import (
	"context"
	"errors"
	"sync/atomic"
	"testing"
	"testing/synctest"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

type discoveryReadResult struct {
	value int
	err   error
}

// Return once the caller has either received a result or blocked on the flight.
func startDiscoveryRead(cache *discoveryCache[int], ctx context.Context, fetch func(context.Context) (int, error)) <-chan discoveryReadResult {
	result := make(chan discoveryReadResult, 1)
	go func() {
		value, err := cache.get(ctx, fetch, func(v int) int { return v })
		result <- discoveryReadResult{value, err}
	}()
	synctest.Wait()
	return result
}

func checkDiscoveryRead(t *testing.T, result <-chan discoveryReadResult, value int, err error) {
	t.Helper()
	synctest.Wait()
	select {
	case got := <-result:
		if got.value != value || !errors.Is(got.err, err) {
			t.Fatalf("discovery returned (%d, %v), want (%d, %v)", got.value, got.err, value, err)
		}
	default:
		t.Fatal("discovery caller is still blocked")
	}
}

func TestDiscoverySingleCallerCancellation(t *testing.T) {
	for _, canceled := range []string{"first", "waiter"} {
		t.Run(canceled, func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var cache discoveryCache[int]
				var calls atomic.Int32
				release := make(chan struct{}, 1)
				defer close(release)
				fetch := func(ctx context.Context) (int, error) {
					calls.Add(1)
					select {
					case <-ctx.Done():
						return 0, ctx.Err()
					case <-release:
						return 42, nil
					}
				}
				first, cancelFirst := context.WithCancel(t.Context())
				defer cancelFirst()
				waiter, cancelWaiter := context.WithCancel(t.Context())
				defer cancelWaiter()
				firstResult := startDiscoveryRead(&cache, first, fetch)
				waiterResult := startDiscoveryRead(&cache, waiter, fetch)
				canceledResult, liveResult := firstResult, waiterResult
				if canceled == "first" {
					cancelFirst()
				} else {
					canceledResult, liveResult = waiterResult, firstResult
					cancelWaiter()
				}
				checkDiscoveryRead(t, canceledResult, 0, context.Canceled)
				release <- struct{}{}
				checkDiscoveryRead(t, liveResult, 42, nil)
				if got := calls.Load(); got != 1 {
					t.Fatalf("controller reads = %d, want 1", got)
				}
			})
		})
	}
}

func TestDiscoveryAbandonedFlight(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cache discoveryCache[int]
		upstream := make(chan context.Context, 1)
		releaseOld := make(chan struct{}, 1)
		defer close(releaseOld)
		oldFetch := func(ctx context.Context) (int, error) {
			upstream <- ctx
			<-releaseOld // The controller can still finish after cancellation.
			return 1, nil
		}
		first, cancelFirst := context.WithCancel(t.Context())
		defer cancelFirst()
		waiter, cancelWaiter := context.WithCancel(t.Context())
		defer cancelWaiter()
		firstResult := startDiscoveryRead(&cache, first, oldFetch)
		oldCtx := <-upstream
		waiterResult := startDiscoveryRead(&cache, waiter, oldFetch)
		cancelWaiter()
		checkDiscoveryRead(t, waiterResult, 0, context.Canceled)
		cancelFirst()
		checkDiscoveryRead(t, firstResult, 0, context.Canceled)
		if oldCtx.Err() != context.Canceled {
			t.Fatalf("abandoned controller read still active: err = %v", oldCtx.Err())
		}

		var calls atomic.Int32
		releaseNew := make(chan struct{}, 1)
		defer close(releaseNew)
		newFetch := func(context.Context) (int, error) {
			calls.Add(1)
			<-releaseNew
			return 2, nil
		}
		fresh := startDiscoveryRead(&cache, t.Context(), newFetch)
		if got := calls.Load(); got != 1 {
			t.Fatalf("new request started %d controller reads while old read is draining, want 1", got)
		}
		releaseOld <- struct{}{}
		synctest.Wait()
		// The old completion must neither publish value 1 nor clear the new flight.
		joined := startDiscoveryRead(&cache, t.Context(), newFetch)
		releaseNew <- struct{}{}
		checkDiscoveryRead(t, fresh, 2, nil)
		checkDiscoveryRead(t, joined, 2, nil)
		checkDiscoveryRead(t, startDiscoveryRead(&cache, t.Context(), newFetch), 2, nil)
		if got := calls.Load(); got != 1 {
			t.Fatalf("replacement controller reads = %d, want 1", got)
		}
	})
}

func TestDiscoveryFlightSharesUpstreamError(t *testing.T) {
	for _, wantErr := range []error{errors.New("controller unavailable"), context.Canceled, context.DeadlineExceeded} {
		t.Run(wantErr.Error(), func(t *testing.T) {
			synctest.Test(t, func(t *testing.T) {
				var cache discoveryCache[int]
				var calls atomic.Int32
				release := make(chan struct{})
				fetch := func(context.Context) (int, error) {
					calls.Add(1)
					<-release
					return 0, wantErr
				}
				first := startDiscoveryRead(&cache, t.Context(), fetch)
				waiter := startDiscoveryRead(&cache, t.Context(), fetch)
				close(release)
				checkDiscoveryRead(t, first, 0, wantErr)
				checkDiscoveryRead(t, waiter, 0, wantErr)
				if got := calls.Load(); got != 1 {
					t.Fatalf("controller reads for concurrent callers = %d, want 1", got)
				}
				checkDiscoveryRead(t, startDiscoveryRead(&cache, t.Context(), fetch), 0, wantErr)
				if got := calls.Load(); got != 2 {
					t.Fatalf("controller reads after independent request = %d, want 2", got)
				}
			})
		})
	}
}

func TestDiscoveryScanSeesChangedMemberDuringCacheLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		controller := &fakeMihomo{proxies: map[string]mihomo.Proxy{
			"ChatGPT": {Name: "ChatGPT", Type: "Selector", All: []string{"JP-01"}},
			"JP-01":   {Name: "JP-01", Type: "Vless"},
			"JP-02":   {Name: "JP-02", Type: "Vless"},
		}}
		manager := NewManager(config.Defaults(), controller, nil)
		request := model.ScanRequest{TargetGroup: "ChatGPT"}
		checkPreview := func() {
			t.Helper()
			group, _, _, err := manager.preflightMembership(t.Context(), request)
			if err != nil || len(group.All) != 1 || group.All[0] != "JP-01" {
				t.Fatalf("preflight membership = %v, %v; want JP-01", group.All, err)
			}
		}
		checkPreview()
		controller.proxies["ChatGPT"].All[0] = "JP-02"
		checkPreview()
		candidates, err := manager.discoverCandidates(t.Context(), request)
		if err != nil || len(candidates) != 1 || candidates[0].Name != "JP-02" {
			t.Fatalf("scan candidates = %v, %v; want JP-02", candidates, err)
		}
	})
}

func TestDiscoveryCacheTwoSecondLifetime(t *testing.T) {
	synctest.Test(t, func(t *testing.T) {
		var cache discoveryCache[int]
		var calls int
		fetch := func(context.Context) (int, error) {
			calls++
			return calls, nil
		}
		read := func(want int) {
			t.Helper()
			checkDiscoveryRead(t, startDiscoveryRead(&cache, t.Context(), fetch), want, nil)
		}
		read(1)
		<-time.After(2*time.Second - time.Nanosecond)
		read(1)
		<-time.After(time.Nanosecond)
		read(2)
	})
}
