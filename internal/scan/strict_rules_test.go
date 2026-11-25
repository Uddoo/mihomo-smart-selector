package scan

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func TestStrictRulesIdentityIncludesHiddenAssertions(t *testing.T) {
	probe := config.StrictProbe{Name: "endpoint", URL: "https://example.com/private", ExpectedStatus: "200", BodyContains: "private-content"}
	id := strictRulesID([]config.StrictProbe{probe})
	if id == "" || id != strictRulesID([]config.StrictProbe{probe}) || strings.Contains(id, "private") {
		t.Fatal("unstable or revealing strict rule identity")
	}
	for _, field := range []string{"body", "url", "status", "restriction"} {
		changed := probe
		switch field {
		case "body":
			changed.BodyContains = "changed"
		case "url":
			changed.URL = "https://example.com/other"
		case "status":
			changed.ExpectedStatus = "204"
		case "restriction":
			changed.RestrictedStatusCodes = []int{403}
		}
		if strictRulesID([]config.StrictProbe{changed}) == id {
			t.Errorf("%s rule change retained old identity", field)
		}
	}
	if strictRulesID(nil) != "" {
		t.Fatal("missing rules must have no identity")
	}
}

func TestSelectionRequiresCurrentStrictEvidence(t *testing.T) {
	old := config.StrictProbe{Name: "content", URL: "https://example.invalid/", ExpectedStatus: "200", BodyContains: "before"}
	updated := old
	updated.BodyContains = "after"
	for _, tc := range []struct {
		name     string
		legacy   bool
		current  []config.StrictProbe
		status   string
		required bool
		allowed  bool
	}{
		{"same ID changed body", false, []config.StrictProbe{updated}, "passed", true, false},
		{"legacy scan without fingerprint", true, []config.StrictProbe{old}, "passed", true, false},
		{"current rules missing", false, nil, "passed", true, false},
		{"same rules failed check", false, []config.StrictProbe{old}, "failed", true, false},
		{"same rules passed check", false, []config.StrictProbe{old}, "passed", true, true},
		{"optional strict checks", true, []config.StrictProbe{updated}, "passed", false, true},
	} {
		t.Run(tc.name, func(t *testing.T) {
			ctx := context.Background()
			fixture, controller, store := newProbeTargetManager(t)
			cfg := fixture.currentConfig()
			profile, err := cfg.ProbeProfileByID(cfg.Scanner.DefaultProbeProfile)
			if err != nil {
				t.Fatal(err)
			}
			profile.StrictProbes, profile.RequireStrict = []config.StrictProbe{old}, true
			summary := fixture.profileSummary(profile)
			if tc.legacy {
				summary.StrictRulesID = "" // Old records could store a pass but not the fingerprint.
			}
			now := time.Now().UTC()
			s := model.Scan{
				ID: "saved", ControllerScope: fixture.bindingScope(), Status: model.ScanComplete,
				StartedAt: now, CompletedAt: &now, Profile: summary,
				Request: model.ScanRequest{TargetGroup: "work", ProfileID: profile.ID, Mode: "quick"},
				Results: []model.NodeResult{{Name: "candidate", Rank: 1, SuccessRate: 1, MeasuredAt: now, StrictVerificationStatus: tc.status}},
			}
			if err := store.CreateScan(ctx, s); err != nil {
				t.Fatal(err)
			}
			if err := store.CompleteScan(ctx, s); err != nil {
				t.Fatal(err)
			}
			profile.StrictProbes, profile.RequireStrict = tc.current, tc.required
			cfg.Scanner.ProbeProfiles = []config.ProbeProfile{profile}
			restarted := NewManager(cfg, fixture.client, store)
			view, err := restarted.Get(ctx, s.ID)
			if err != nil || len(view.Results) != 1 {
				t.Fatalf("historical result unavailable: %+v, %v", view, err)
			}
			reason := view.Results[0].SelectionReason
			event, err := restarted.SelectRequest(ctx, s.ID, "candidate", "new-selection")
			if tc.allowed {
				if reason != "" || err != nil || event.Status != "confirmed" {
					t.Errorf("valid selection: reason=%q event=%+v err=%v", reason, event, err)
				}
			} else if reason == "" || err == nil || err.Error() != reason {
				t.Errorf("display and write must both reject stale strict evidence: reason=%q err=%v", reason, err)
			}
			audits, err := store.ListSwitches(ctx, 10)
			_, puts := controller.counts()
			want := 0
			if tc.allowed {
				want = 1
			}
			if err != nil || len(audits) != want || puts != want {
				t.Fatalf("got %d audits, %d PUTs; want %d of each; err=%v", len(audits), puts, want, err)
			}
			if tc.required && tc.allowed {
				profile.StrictProbes = []config.StrictProbe{updated}
				cfg.Scanner.ProbeProfiles = []config.ProbeProfile{profile}
				reloaded := NewManager(cfg, fixture.client, store)
				replay, err := reloaded.SelectRequest(ctx, s.ID, "candidate", "new-selection")
				if _, writes := controller.counts(); err != nil || replay.ID != event.ID || replay.Status != "confirmed" || writes != 1 {
					t.Fatalf("rule changes must preserve idempotent replay: %+v writes=%d err=%v", replay, writes, err)
				}
			}
		})
	}
}
