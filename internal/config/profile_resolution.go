package config

import (
	"fmt"
	"strings"
	"unicode"
)

func (c *Config) applyProbeProfileOverrides() error {
	for id, override := range c.Scanner.ProbeProfileOverrides {
		found := false
		for index := range c.Scanner.ProbeProfiles {
			profile := &c.Scanner.ProbeProfiles[index]
			if !strings.EqualFold(profile.ID, strings.TrimSpace(id)) {
				continue
			}
			found = true
			if strings.TrimSpace(override.Label) != "" {
				profile.Label = override.Label
			}
			if strings.TrimSpace(override.Description) != "" {
				profile.Description = override.Description
			}
			if len(override.Probes) > 0 {
				profile.Probes = append([]Probe(nil), override.Probes...)
			}
			if len(override.StrictProbes) > 0 {
				profile.StrictProbes = append([]StrictProbe(nil), override.StrictProbes...)
			}
			if override.RequireStrict != nil {
				profile.RequireStrict = *override.RequireStrict
			}
			if override.RequireRegion != nil {
				profile.RequireRegion = *override.RequireRegion
			}
			if len(override.ExpectedRegions) > 0 {
				profile.ExpectedRegions = append([]string(nil), override.ExpectedRegions...)
			}
			if strings.TrimSpace(override.TransportScope) != "" {
				profile.TransportScope = override.TransportScope
			}
			if override.RequiresConfiguration != nil {
				profile.RequiresConfiguration = *override.RequiresConfiguration
			}
			if strings.TrimSpace(override.SetupHint) != "" || override.RequiresConfiguration != nil && !*override.RequiresConfiguration {
				profile.SetupHint = override.SetupHint
			}
			break
		}
		if !found {
			return fmt.Errorf("probe_profile_overrides references unknown profile %q", id)
		}
	}
	return nil
}

// ResolveProbeProfile maps the user-facing selector name to a semantic probe
// profile. Selector decorations (emoji, spaces, '+' and punctuation) are
// ignored so an OpenClash label such as "🤖 ChatGPT" maps to "ChatGPT".
func (c Config) ResolveProbeProfile(groupName string) (ProbeProfile, error) {
	if len(c.Scanner.ProbeProfiles) == 0 {
		return ProbeProfile{ID: "legacy", Label: "Legacy global probes", Description: "Uses scanner.probes for every selector.", Probes: append([]Probe(nil), c.Scanner.Probes...), TransportScope: "HTTP latency only"}, nil
	}
	key := normaliseGroupName(groupName)
	for _, profile := range c.Scanner.ProbeProfiles {
		for _, group := range profile.GroupNames {
			if normaliseGroupName(group) == key {
				return cloneProbeProfile(profile), nil
			}
		}
	}
	defaultID := strings.TrimSpace(c.Scanner.DefaultProbeProfile)
	for _, profile := range c.Scanner.ProbeProfiles {
		if strings.EqualFold(profile.ID, defaultID) {
			return cloneProbeProfile(profile), nil
		}
	}
	return ProbeProfile{}, fmt.Errorf("no probe profile is configured for selector %q", groupName)
}

// ProbeProfileByID selects a service independently of the user's group naming.
func (c Config) ProbeProfileByID(id string) (ProbeProfile, error) {
	if id == "legacy" && len(c.Scanner.ProbeProfiles) == 0 {
		return c.ResolveProbeProfile("")
	}
	for _, profile := range c.Scanner.ProbeProfiles {
		if strings.EqualFold(strings.TrimSpace(id), strings.TrimSpace(profile.ID)) {
			return cloneProbeProfile(profile), nil
		}
	}
	return ProbeProfile{}, fmt.Errorf("unknown service profile %q; select an available service", id)
}

func (c Config) SuggestedProfile(groupName string) string {
	for _, profile := range c.Scanner.ProbeProfiles {
		for _, name := range profile.GroupNames {
			if normaliseGroupName(name) == normaliseGroupName(groupName) {
				return profile.ID
			}
		}
	}
	return ""
}

func normaliseGroupName(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func cloneProbeProfile(input ProbeProfile) ProbeProfile {
	output := input
	output.GroupNames = append([]string(nil), input.GroupNames...)
	output.Probes = append([]Probe(nil), input.Probes...)
	output.StrictProbes = append([]StrictProbe(nil), input.StrictProbes...)
	output.ExpectedRegions = append([]string(nil), input.ExpectedRegions...)
	return output
}
