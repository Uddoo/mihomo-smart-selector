package scan

import (
	"context"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

func (m *Manager) verifyEgress(ctx context.Context, proxies map[string]mihomo.Proxy, results []model.NodeResult, scanID string) {
	probeSelector, exists := proxies[m.currentConfig().EgressVerification.SelectorGroup]
	if !exists || !strings.EqualFold(probeSelector.Type, "Selector") {
		for index := range results {
			results[index].EgressError = "configured probe selector is unavailable"
		}
		return
	}
	defer m.restoreProbeSelector(probeSelector.Name, probeSelector.Now)
	proxyURL, err := url.Parse(m.currentConfig().EgressVerification.ProxyURL)
	if err != nil {
		for index := range results {
			results[index].EgressError = "configured probe listener URL is invalid"
		}
		return
	}
	httpClient := &http.Client{
		Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS+1000) * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true,
			Proxy:             http.ProxyURL(proxyURL),
			DialContext:       (&net.Dialer{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS) * time.Millisecond}).DialContext,
		},
	}
	defer httpClient.CloseIdleConnections()
	for index := range results {
		if ctx.Err() != nil {
			return
		}
		if !contains(probeSelector.All, results[index].Name) {
			results[index].EgressError = "candidate is not a member of configured probe selector"
			continue
		}
		if err := m.client.Select(ctx, probeSelector.Name, results[index].Name); err != nil {
			results[index].EgressError = "could not select candidate in probe selector"
			continue
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, m.currentConfig().EgressVerification.TraceURL, nil)
		if err != nil {
			results[index].EgressError = "could not create egress probe request"
			continue
		}
		response, err := httpClient.Do(request)
		if err != nil {
			results[index].EgressError = "egress trace request failed"
			continue
		}
		body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
		response.Body.Close()
		if readErr != nil || response.StatusCode != http.StatusOK {
			results[index].EgressError = "egress trace response was not usable"
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			key, value, found := strings.Cut(line, "=")
			if found && key == "loc" && len(value) == 2 {
				results[index].VerifiedRegion = strings.ToUpper(value)
			}
		}
		if results[index].VerifiedRegion == "" {
			results[index].EgressError = "egress trace did not include a country code"
		}
		copy := results[index]
		m.publish(scanID, Event{Kind: "egress-verified", Message: "egress verification completed: " + results[index].Name, At: time.Now().UTC(), Result: &copy})
	}
}

// verifyStrict performs status-code and optional body checks through a
// dedicated, user-configured selector. The target business selector is never
// changed. Disabling this feature is the default because a router must first
// provide an isolated probe selector and local proxy listener.
func (m *Manager) verifyStrict(ctx context.Context, proxies map[string]mihomo.Proxy, profile config.ProbeProfile, results []model.NodeResult, scanID string) {
	if len(profile.StrictProbes) == 0 {
		return
	}
	verification := m.currentConfig().Scanner.StrictVerification
	if !verification.Enabled {
		for index := range results {
			results[index].StrictVerificationStatus = "not_configured"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	rankNodes(results)
	for index := verification.MaxCandidates; index < len(results); index++ {
		results[index].StrictVerificationStatus = "not_run_limit"
		results[index].RestrictionStatus = "not_checked"
	}
	results = results[:min(len(results), verification.MaxCandidates)]
	probeSelector, exists := proxies[verification.SelectorGroup]
	if !exists || !strings.EqualFold(probeSelector.Type, "Selector") {
		for index := range results {
			results[index].StrictVerificationStatus = "probe_selector_unavailable"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	proxyURL, err := url.Parse(verification.ProxyURL)
	if err != nil {
		for index := range results {
			results[index].StrictVerificationStatus = "probe_proxy_invalid"
			results[index].RestrictionStatus = "not_checked"
		}
		return
	}
	transport := &http.Transport{
		Proxy:             http.ProxyURL(proxyURL),
		DisableKeepAlives: true,
		DialContext:       (&net.Dialer{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS) * time.Millisecond}).DialContext,
	}
	client := &http.Client{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS+1000) * time.Millisecond, Transport: transport}
	defer transport.CloseIdleConnections()
	if probeSelector.Now != "" {
		defer m.restoreProbeSelector(probeSelector.Name, probeSelector.Now)
	}
	for index := range results {
		result := &results[index]
		if ctx.Err() != nil {
			return
		}
		if !contains(probeSelector.All, result.Name) {
			result.StrictVerificationStatus = "candidate_not_in_probe_selector"
			result.RestrictionStatus = "not_checked"
			continue
		}
		if err := m.client.Select(ctx, probeSelector.Name, result.Name); err != nil {
			result.StrictVerificationStatus = "probe_selector_switch_failed"
			result.RestrictionStatus = "not_checked"
			continue
		}
		for _, probe := range profile.StrictProbes {
			if ctx.Err() != nil {
				return
			}
			result.StrictChecks = append(result.StrictChecks, m.runStrictCheck(ctx, client, probe))
		}
		applyStrictOutcome(result)
		copy := *result
		m.publish(scanID, Event{Kind: "strict-verified", Message: "strict service verification completed: " + result.Name, At: time.Now().UTC(), Result: &copy})
	}
}

func (m *Manager) runStrictCheck(parent context.Context, client *http.Client, probe config.StrictProbe) model.StrictCheck {
	check := model.StrictCheck{Probe: probe.Name, ExpectedStatus: probe.ExpectedStatus, Status: "failed"}
	ctx, cancel := context.WithTimeout(parent, time.Duration(m.currentConfig().Scanner.TimeoutMS+500)*time.Millisecond)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodGet, probe.URL, nil)
	if err != nil {
		check.Error = "could not create strict probe request"
		return check
	}
	response, err := client.Do(request)
	if err != nil {
		check.Error = "strict probe request failed"
		return check
	}
	body, readErr := io.ReadAll(io.LimitReader(response.Body, 64<<10))
	response.Body.Close()
	check.ObservedStatus = response.StatusCode
	if readErr != nil {
		check.Error = "could not read strict probe response"
		return check
	}
	expected, _ := strconv.Atoi(probe.ExpectedStatus)
	if containsStatus(probe.RestrictedStatusCodes, response.StatusCode) {
		check.Status = "restricted"
		return check
	}
	if response.StatusCode != expected {
		check.Error = fmt.Sprintf("expected HTTP %d, received %d", expected, response.StatusCode)
		return check
	}
	if probe.BodyContains != "" {
		matched := strings.Contains(string(body), probe.BodyContains)
		check.BodyMatched = &matched
		if !matched {
			check.Error = "expected response content was not present"
			return check
		}
	}
	check.Status = "passed"
	return check
}

func (m *Manager) restoreProbeSelector(groupName, previous string) {
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	_ = m.client.Select(ctx, groupName, previous)
}

func applyStrictOutcome(result *model.NodeResult) {
	if len(result.StrictChecks) == 0 {
		return
	}
	passed := true
	for _, check := range result.StrictChecks {
		if check.Status == "restricted" {
			result.StrictVerificationStatus = "restricted"
			result.RestrictionStatus = "restricted"
			return
		}
		if check.Status != "passed" {
			passed = false
		}
	}
	if passed {
		result.StrictVerificationStatus = "passed"
		result.RestrictionStatus = "not_restricted"
		return
	}
	result.StrictVerificationStatus = "failed"
	result.RestrictionStatus = "unknown"
}

func assessResult(result *model.NodeResult, profile config.ProbeProfile, strictEnabled bool) {
	switch {
	case result.SuccessRate >= 1:
		result.ReachabilityStatus = "available"
	case result.SuccessRate > 0:
		result.ReachabilityStatus = "partial"
	default:
		result.ReachabilityStatus = "unavailable"
	}
	if len(profile.StrictProbes) == 0 {
		result.StrictVerificationStatus = "not_requested"
		result.RestrictionStatus = "not_checked"
	} else if !strictEnabled && result.StrictVerificationStatus == "" {
		result.StrictVerificationStatus = "not_configured"
		result.RestrictionStatus = "not_checked"
	}
	if len(profile.ExpectedRegions) == 0 {
		result.RegionVerificationStatus = "not_configured"
	} else if result.VerifiedRegion == "" {
		result.RegionVerificationStatus = "unverified"
	} else if containsFold(profile.ExpectedRegions, result.VerifiedRegion) {
		result.RegionVerificationStatus = "matched"
	} else {
		result.RegionVerificationStatus = "mismatch"
	}
	if profile.TransportScope == "" {
		result.TransportStatus = "latency_only"
	} else {
		result.TransportStatus = profile.TransportScope
	}
}
