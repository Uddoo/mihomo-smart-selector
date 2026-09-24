package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"strconv"
	"strings"
)

func (c Config) Validate() error {
	host, _, err := net.SplitHostPort(c.HTTP.Listen)
	if err != nil || host == "" {
		return fmt.Errorf("http.listen must be host:port: %q", c.HTTP.Listen)
	}
	loopback := host == "localhost"
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		loopback = ip.IsLoopback()
	}
	if !loopback && !c.HTTP.AllowUnauthenticatedLAN && strings.TrimSpace(c.HTTP.APIToken) == "" {
		return fmt.Errorf("http.api_token is required when http.listen is not loopback")
	}
	if !loopback && len(c.HTTP.AllowedCIDRs) == 0 {
		return fmt.Errorf("http.allowed_cidrs is required when http.listen is not loopback")
	}
	for _, value := range c.HTTP.AllowedCIDRs {
		if _, err := netip.ParsePrefix(value); err != nil {
			return fmt.Errorf("invalid http.allowed_cidrs entry %q: %w", value, err)
		}
	}
	if _, err := ParseControllerURL(c.Mihomo.Controller); err != nil {
		return fmt.Errorf("mihomo.controller: %w", err)
	}
	if c.Mihomo.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("mihomo.request_timeout_seconds must be positive")
	}
	if strings.TrimSpace(c.Mihomo.SecretEnv) == "" {
		return fmt.Errorf("mihomo.secret_env must not be empty")
	}
	if c.Storage.Path == "" {
		return fmt.Errorf("storage.path must not be empty")
	}
	if c.Scanner.Concurrency < 1 || c.Scanner.Concurrency > 16 {
		return fmt.Errorf("scanner.concurrency must be in 1..16")
	}
	if c.Scanner.BatchSize < 1 || c.Scanner.BatchSize > 100 {
		return fmt.Errorf("scanner.batch_size must be in 1..100")
	}
	if c.Scanner.MaxTotalCandidates < 1 || c.Scanner.MaxTotalCandidates > 1000 {
		return fmt.Errorf("scanner.max_total_candidates must be in 1..1000")
	}
	if c.Scanner.MaxActiveScans < 1 || c.Scanner.MaxActiveScans > 8 {
		return fmt.Errorf("scanner.max_active_scans must be in 1..8")
	}
	r := c.Storage.Retention
	if r.ScanDays < 1 || r.ScanDays > 3650 || r.AuditDays < 1 || r.AuditDays > 3650 || r.MaxScans < 1 || r.MaxScans > 100000 || r.MaxAudit < 1 || r.MaxAudit > 100000 {
		return fmt.Errorf("invalid storage retention: days must be 1..3650, counts 1..100000")
	}
	if c.Scanner.RefineTopK < 1 || c.Scanner.RefineTopK > 1000 {
		return fmt.Errorf("scanner.refine_top_k must be in 1..1000")
	}
	if c.Scanner.ResultMaxAgeSeconds < 30 || c.Scanner.ResultMaxAgeSeconds > 86400 {
		return fmt.Errorf("scanner.result_max_age_seconds must be in 30..86400")
	}
	if c.Scanner.Samples < 1 || c.Scanner.Samples > 10 {
		return fmt.Errorf("scanner.samples must be in 1..10")
	}
	if c.Scanner.TimeoutMS < 100 || c.Scanner.TimeoutMS > 60000 {
		return fmt.Errorf("scanner.timeout_ms must be in 100..60000")
	}
	if c.Scanner.MinSuccessRate < 0 || c.Scanner.MinSuccessRate > 1 {
		return fmt.Errorf("scanner.min_success_rate must be in 0..1")
	}
	if c.Scanner.MedianTargetMS <= 0 || c.Scanner.P95TargetMS <= 0 || c.Scanner.JitterTargetMS <= 0 {
		return fmt.Errorf("scanner score targets must be positive")
	}
	if len(c.Scanner.Probes) == 0 && len(c.Scanner.ProbeProfiles) == 0 {
		return fmt.Errorf("scanner requires scanner.probes or scanner.probe_profiles")
	}
	if err := validateProbes("scanner.probes", c.Scanner.Probes); err != nil {
		return err
	}
	if err := c.validateProbeProfiles(); err != nil {
		return err
	}
	if err := c.validateStrictVerification(); err != nil {
		return err
	}
	regionCodes := map[string]bool{}
	for _, region := range c.Regions {
		code := strings.ToUpper(strings.TrimSpace(region.Code))
		if regionCodes[code] {
			return fmt.Errorf("duplicate region code %q", code)
		}
		regionCodes[code] = true
	}
	regionCodes = map[string]bool{}
	for _, region := range MergeRegions(c.Regions) {
		if region.Code == "" || region.Name == "" || len(region.Aliases) == 0 {
			return fmt.Errorf("every region needs code, name, and at least one alias")
		}
		if regionCodes[region.Code] {
			return fmt.Errorf("duplicate region code %q", region.Code)
		}
		regionCodes[region.Code] = true
	}
	for name, code := range c.RegionOverrides {
		if strings.TrimSpace(name) == "" || !regionCodes[strings.ToUpper(strings.TrimSpace(code))] {
			return fmt.Errorf("region override %q references unknown region %q", name, code)
		}
	}
	if c.EgressVerification.Enabled {
		if c.EgressVerification.SelectorGroup == "" || c.EgressVerification.ProxyURL == "" || c.EgressVerification.TraceURL == "" {
			return fmt.Errorf("enabled egress_verification requires selector_group, proxy_url, and trace_url")
		}
		if _, err := url.ParseRequestURI(c.EgressVerification.ProxyURL); err != nil {
			return fmt.Errorf("invalid egress_verification.proxy_url: %w", err)
		}
		if _, err := url.ParseRequestURI(c.EgressVerification.TraceURL); err != nil {
			return fmt.Errorf("invalid egress_verification.trace_url: %w", err)
		}
	}
	return nil
}

func (c Config) validateProbeProfiles() error {
	if len(c.Scanner.ProbeProfiles) == 0 {
		return nil
	}
	profileIDs := map[string]bool{}
	groupOwners := map[string]string{}
	for _, profile := range c.Scanner.ProbeProfiles {
		profile.ID = strings.TrimSpace(profile.ID)
		if profile.ID == "" || strings.TrimSpace(profile.Label) == "" {
			return fmt.Errorf("every scanner probe profile needs id and label")
		}
		key := strings.ToLower(profile.ID)
		if profileIDs[key] {
			return fmt.Errorf("duplicate scanner probe profile id %q", profile.ID)
		}
		profileIDs[key] = true
		if profile.TransportScope == "" {
			return fmt.Errorf("probe profile %q needs transport_scope", profile.ID)
		}
		if profile.RequiresConfiguration {
			if strings.TrimSpace(profile.SetupHint) == "" {
				return fmt.Errorf("probe profile %q requires setup_hint when configuration is required", profile.ID)
			}
		} else if len(profile.Probes) == 0 {
			return fmt.Errorf("probe profile %q must contain at least one probe", profile.ID)
		}
		if err := validateProbes("probe profile "+profile.ID, profile.Probes); err != nil {
			return err
		}
		if err := validateStrictProbes(profile); err != nil {
			return err
		}
		if profile.RequireStrict && len(profile.StrictProbes) == 0 {
			return fmt.Errorf("profile %q requires strict probes for require_strict", profile.ID)
		}
		if profile.RequireRegion && len(profile.ExpectedRegions) == 0 {
			return fmt.Errorf("profile %q requires expected_regions for require_region", profile.ID)
		}
		for _, group := range profile.GroupNames {
			groupKey := normaliseGroupName(group)
			if groupKey == "" {
				return fmt.Errorf("probe profile %q has an empty group name", profile.ID)
			}
			if owner, exists := groupOwners[groupKey]; exists {
				return fmt.Errorf("group %q belongs to both probe profiles %q and %q", group, owner, profile.ID)
			}
			groupOwners[groupKey] = profile.ID
		}
		for _, region := range profile.ExpectedRegions {
			if strings.TrimSpace(region) == "" {
				return fmt.Errorf("probe profile %q has an empty expected region", profile.ID)
			}
		}
	}
	if !profileIDs[strings.ToLower(strings.TrimSpace(c.Scanner.DefaultProbeProfile))] {
		return fmt.Errorf("scanner.default_probe_profile %q is not defined", c.Scanner.DefaultProbeProfile)
	}
	return nil
}

func (c Config) validateStrictVerification() error {
	verification := c.Scanner.StrictVerification
	if !verification.Enabled {
		return nil
	}
	if strings.TrimSpace(verification.SelectorGroup) == "" || strings.TrimSpace(verification.ProxyURL) == "" {
		return fmt.Errorf("enabled scanner.strict_verification requires selector_group and proxy_url")
	}
	if verification.MaxCandidates < 1 || verification.MaxCandidates > c.Scanner.MaxTotalCandidates {
		return fmt.Errorf("scanner.strict_verification.max_candidates must be in 1..scanner.max_total_candidates")
	}
	proxyURL, err := url.ParseRequestURI(verification.ProxyURL)
	if err != nil || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") || proxyURL.Host == "" {
		return fmt.Errorf("scanner.strict_verification.proxy_url must be an absolute HTTP(S) URL")
	}
	return nil
}

func validateProbes(scope string, probes []Probe) error {
	probeNames := map[string]bool{}
	for _, probe := range probes {
		if strings.TrimSpace(probe.Name) == "" || strings.TrimSpace(probe.ExpectedStatus) == "" {
			return fmt.Errorf("every %s probe needs name and expected_status", scope)
		}
		if probeNames[probe.Name] {
			return fmt.Errorf("duplicate %s probe name %q", scope, probe.Name)
		}
		probeNames[probe.Name] = true
		if err := validateHTTPSURL(probe.URL); err != nil {
			return fmt.Errorf("probe %q in %s: %w", probe.Name, scope, err)
		}
		if _, err := validHTTPStatus(probe.ExpectedStatus); err != nil {
			return fmt.Errorf("probe %q in %s: %w", probe.Name, scope, err)
		}
	}
	return nil
}

func validateStrictProbes(profile ProbeProfile) error {
	names := map[string]bool{}
	for _, probe := range profile.StrictProbes {
		if strings.TrimSpace(probe.Name) == "" || strings.TrimSpace(probe.ExpectedStatus) == "" {
			return fmt.Errorf("every strict probe in profile %q needs name and expected_status", profile.ID)
		}
		if names[probe.Name] {
			return fmt.Errorf("duplicate strict probe name %q in profile %q", probe.Name, profile.ID)
		}
		names[probe.Name] = true
		if err := validateHTTPSURL(probe.URL); err != nil {
			return fmt.Errorf("strict probe %q in profile %q: %w", probe.Name, profile.ID, err)
		}
		if _, err := validHTTPStatus(probe.ExpectedStatus); err != nil {
			return fmt.Errorf("strict probe %q in profile %q: %w", probe.Name, profile.ID, err)
		}
		for _, status := range probe.RestrictedStatusCodes {
			if status < 100 || status > 599 {
				return fmt.Errorf("strict probe %q in profile %q has invalid restricted HTTP status %d", probe.Name, profile.ID, status)
			}
		}
	}
	return nil
}

func validateHTTPSURL(value string) error {
	u, err := url.ParseRequestURI(value)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("must use an absolute HTTPS URL")
	}
	return nil
}

func validHTTPStatus(value string) (int, error) {
	status, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || status < 100 || status > 599 {
		return 0, fmt.Errorf("expected_status must be an HTTP status in 100..599")
	}
	return status, nil
}
