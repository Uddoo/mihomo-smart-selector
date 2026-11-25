package scan

import (
	"context"
	"testing"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

// Pre-upgrade rows contain a country but no indication of which Controller
// supplied it. Today's Controller configuration cannot establish that retroactively.
func TestLegacyEgressRegionCannotAuthorizeSelection(t *testing.T) {
	ctx := context.Background()
	fixture, controller, store := newProbeTargetManager(t)
	cfg := fixture.currentConfig()
	profile, err := cfg.ProbeProfileByID(cfg.Scanner.DefaultProbeProfile)
	if err != nil {
		t.Fatal(err)
	}
	profile.RequireRegion, profile.ExpectedRegions = true, []string{"JP"}
	cfg.Scanner.ProbeProfiles = []config.ProbeProfile{profile}
	m := NewManager(cfg, fixture.client, store)
	now := time.Now().UTC()
	s := model.Scan{
		ID: "old-region", ControllerScope: m.bindingScope(), Status: model.ScanComplete,
		StartedAt: now, CompletedAt: &now, Profile: m.profileSummary(profile),
		Request: model.ScanRequest{TargetGroup: "work", ProfileID: profile.ID, Mode: "quick"},
		Results: []model.NodeResult{{Name: "candidate", Rank: 1, MeasuredAt: now, SuccessRate: 1, VerifiedRegion: "JP"}},
	}
	if err := store.CreateScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	if err := store.CompleteScan(ctx, s); err != nil {
		t.Fatal(err)
	}
	view, err := m.Get(ctx, s.ID)
	if err != nil || len(view.Results) != 1 || view.Results[0].VerifiedRegion != "JP" {
		t.Fatalf("legacy evidence should remain readable: %+v err=%v", view, err)
	}
	reason := view.Results[0].SelectionReason
	if _, err := m.SelectRequest(ctx, s.ID, "candidate", "new"); err == nil || reason == "" || err.Error() != reason {
		t.Errorf("legacy region permitted or inconsistent: display=%q err=%v", reason, err)
	}
	audits, err := store.ListSwitches(ctx, 10)
	if _, writes := controller.counts(); err != nil || len(audits) != 0 || writes != 0 {
		t.Fatalf("unverified region: audits=%d writes=%d err=%v", len(audits), writes, err)
	}

	// Exercise the gate independently of how the evidence is produced. Only
	// known observation methods qualify; non-region-gated profiles are unchanged.
	for _, tc := range []struct {
		source   string
		required bool
		allowed  bool
	}{
		{"primary", true, true},
		{"isolated_identity_v1", true, true},
		{"unrecognized", true, false},
		{"", false, true},
	} {
		row := view.Results[0]
		row.EgressEvidenceSource = tc.source
		profile.RequireRegion = tc.required
		reason := m.selectionReason(s, row, selectionPolicyFor(profile), now)
		if (reason == "") != tc.allowed {
			t.Errorf("source=%q required=%t allowed=%t: %q", tc.source, tc.required, tc.allowed, reason)
		}
	}
}
