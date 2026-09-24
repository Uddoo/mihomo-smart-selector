package scan

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) profileFor(request model.ScanRequest) (config.ProbeProfile, error) {
	profile, err := m.resolveService(context.Background(), request)
	if err != nil {
		return config.ProbeProfile{}, err
	}
	if profile.RequiresConfiguration {
		return config.ProbeProfile{}, fmt.Errorf("probe profile %q needs configuration: %s", profile.Label, profile.SetupHint)
	}
	if len(profile.Probes) == 0 {
		return config.ProbeProfile{}, fmt.Errorf("probe profile %q has no reachable probes", profile.Label)
	}
	return profile, nil
}

func (m *Manager) profileSummary(profile config.ProbeProfile) model.ProbeProfileSummary {
	targets := make([]model.ProbeTargetSummary, 0, len(profile.Probes)+len(profile.StrictProbes))
	for _, probe := range profile.Probes {
		targets = append(targets, model.ProbeTargetSummary{
			Name: probe.Name, ExpectedStatus: probe.ExpectedStatus, Kind: "reachability", AddressVisible: profile.ExposeTargetAddresses,
		})
		if profile.ExposeTargetAddresses {
			targets[len(targets)-1].Address = probe.URL
		}
	}
	for _, probe := range profile.StrictProbes {
		targets = append(targets, model.ProbeTargetSummary{
			Name: probe.Name, ExpectedStatus: probe.ExpectedStatus, Kind: "strict", AddressVisible: profile.ExposeTargetAddresses,
		})
		if profile.ExposeTargetAddresses {
			targets[len(targets)-1].Address = probe.URL
		}
	}
	return model.ProbeProfileSummary{
		StrictRulesID: strictRulesID(profile.StrictProbes),
		RequireStrict: profile.RequireStrict, RequireRegion: profile.RequireRegion,
		ID: profile.ID, Label: profile.Label, Description: profile.Description,
		ProbeCount: len(profile.Probes), StrictProbeCount: len(profile.StrictProbes),
		StrictVerificationAvailable: len(profile.StrictProbes) > 0 && m.currentConfig().Scanner.StrictVerification.Enabled,
		RequiresConfiguration:       profile.RequiresConfiguration, SetupHint: profile.SetupHint,
		ExpectedRegions: append([]string(nil), profile.ExpectedRegions...), TransportScope: profile.TransportScope,
		Targets: targets,
	}
}

// Include non-public body and restriction rules without exposing their content.
// Old persisted scans have no ID and must not be presented as current evidence.
func strictRulesID(probes []config.StrictProbe) string {
	if len(probes) == 0 {
		return ""
	}
	data, _ := json.Marshal(probes)
	digest := sha256.Sum256(data)
	return "strict-v1:" + hex.EncodeToString(digest[:])
}
