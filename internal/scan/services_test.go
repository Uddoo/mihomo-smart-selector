package scan

import (
	"context"
	"path/filepath"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestServicesAdaptDifferentGroupNamesAndPreserveBindings(t *testing.T) {
	for _, name := range []string{"🤖 ChatGPT", "海外工作 / Office"} {
		t.Run(name, func(t *testing.T) {
			ctx := context.Background()
			cfg := config.Defaults()
			path := filepath.Join(t.TempDir(), "bindings.db")
			store, err := history.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{
				name:  {Name: name, Type: "Selector", Now: "old", All: []string{"old", "new"}},
				"old": {Name: "old", Type: "VLESS"}, "new": {Name: "new", Type: "VLESS"},
			}, delays: map[string]int{"old": 250, "new": 100}}
			manager := NewManager(cfg, fake, store)
			if err := manager.SetBinding(ctx, model.ServiceBinding{Group: name, ProfileID: "youtube"}); err != nil {
				t.Fatal(err)
			}
			if err := store.Close(); err != nil {
				t.Fatal(err)
			}
			store, err = history.Open(path)
			if err != nil {
				t.Fatal(err)
			}
			defer store.Close()
			manager = NewManager(cfg, fake, store)
			request := model.ScanRequest{TargetGroup: name, Mode: "quick"}
			preview, err := manager.Preflight(ctx, request)
			if err != nil || !preview.Ready || preview.Profile.ID != "youtube" {
				t.Fatalf("persisted binding: %+v, %v", preview, err)
			}
			request.ProfileID = "github"
			explicit, err := manager.Preflight(ctx, request)
			if err != nil || explicit.Profile.ID != "github" {
				t.Fatalf("explicit selection: %+v, %v", explicit, err)
			}
			request.ProfileID = "missing"
			if _, err := manager.Preflight(ctx, request); err == nil {
				t.Fatal("unknown explicit profile silently fell back")
			}
			request.ProfileID = ""
			started, err := manager.Start(request)
			if err != nil {
				t.Fatal(err)
			}
			if started.Request.ProfileID != "youtube" {
				t.Fatal("scan did not freeze resolved profile")
			}
			if err := manager.SetBinding(ctx, model.ServiceBinding{Group: name, ProfileID: "github"}); err != nil {
				t.Fatal(err)
			}
			deadline := time.Now().Add(5 * time.Second)
			var completed model.Scan
			for {
				completed, err = manager.Get(ctx, started.ID)
				if err != nil {
					t.Fatal(err)
				}
				if completed.Status != model.ScanRunning {
					break
				}
				if time.Now().After(deadline) {
					t.Fatal("scan timed out")
				}
				time.Sleep(10 * time.Millisecond)
			}
			if completed.Status != model.ScanComplete || completed.Profile.ID != "youtube" || len(completed.Results) != 2 {
				t.Fatalf("scan: %+v", completed)
			}
			if fake.selected != "" {
				t.Fatal("scan changed selection")
			}
			if _, err := manager.Select(ctx, started.ID, "new"); err != nil {
				t.Fatal(err)
			}
			if fake.selected != "new" {
				t.Fatal("manual switch not applied")
			}

			catalog, err := manager.Services(ctx)
			if err != nil || len(catalog.Bindings) != 1 || catalog.Bindings[0].Status != "valid" {
				t.Fatalf("catalog: %+v, %v", catalog, err)
			}
			// A different endpoint must not inherit this controller's bindings.
			other := cfg
			other.Mihomo.Controller = "http://other-router:9090"
			otherCatalog, err := NewManager(other, fake, store).Services(ctx)
			if err != nil || len(otherCatalog.Bindings) != 0 {
				t.Fatalf("controller isolation: %+v %v", otherCatalog, err)
			}
			delete(fake.proxies, name)
			catalog, err = manager.Services(ctx)
			if err != nil || catalog.Bindings[0].Status != "group_missing" {
				t.Fatalf("missing group: %+v %v", catalog, err)
			}
			if err := manager.SetBinding(ctx, model.ServiceBinding{Group: name, ProfileID: "youtube"}); err == nil {
				t.Fatal("bound missing group")
			}
			if err := manager.SetBinding(ctx, model.ServiceBinding{Group: name}); err != nil {
				t.Fatal(err)
			}
			catalog, err = manager.Services(ctx)
			if err != nil || len(catalog.Bindings) != 0 {
				t.Fatal("failed to remove stale binding")
			}
		})
	}
}

func TestMissingProfileBindingDoesNotSilentlyFallback(t *testing.T) {
	cfg := config.Defaults()
	store, err := history.Open(filepath.Join(t.TempDir(), "bindings.db"))
	if err != nil {
		t.Fatal(err)
	}
	defer store.Close()
	fake := &fakeMihomo{proxies: map[string]mihomo.Proxy{"Work": {Name: "Work", Type: "Selector"}}}
	manager := NewManager(cfg, fake, store)
	ctx := context.Background()
	if err := store.SetBinding(ctx, manager.bindingScope(), model.ServiceBinding{Group: "Work", ProfileID: "deleted-custom"}); err != nil {
		t.Fatal(err)
	}
	catalog, err := manager.Services(ctx)
	if err != nil || catalog.Bindings[0].Status != "profile_missing" {
		t.Fatalf("catalog: %+v %v", catalog, err)
	}
	if _, err := manager.Preflight(ctx, model.ScanRequest{TargetGroup: "Work"}); err == nil {
		t.Fatal("stale profile silently fell back")
	}
}
