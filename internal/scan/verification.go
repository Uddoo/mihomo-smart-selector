package scan

import (
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

func (m *Manager) verifyEgress(ctx context.Context, results []model.NodeResult, scanID string) (warning *model.ScanWarning) {
	if len(results) == 0 {
		return nil
	}
	for i := range results {
		results[i].VerifiedRegion = ""
		results[i].EgressError = probeSelectionMessage("probe_selector_unconfirmed")
	}
	verification := m.currentConfig().EgressVerification
	proxyURL, err := url.Parse(verification.ProxyURL)
	if err != nil {
		for i := range results {
			results[i].EgressError = "configured probe listener URL is invalid"
		}
		return nil
	}
	session, code := m.beginProbeSelector(ctx, verification.SelectorGroup)
	if code != "" {
		for i := range results {
			results[i].EgressError = probeSelectionMessage(code)
		}
		return nil
	}
	defer func() { warning = session.restore("egress") }()
	httpClient := &http.Client{
		Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS+1000) * time.Millisecond,
		Transport: &http.Transport{
			DisableKeepAlives: true, Proxy: http.ProxyURL(proxyURL),
			DialContext: (&net.Dialer{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS) * time.Millisecond}).DialContext,
		},
	}
	defer httpClient.CloseIdleConnections()
	for i := range results {
		result := &results[i]
		if ctx.Err() != nil {
			return nil
		}
		if code := session.selectCandidate(ctx, result.Name); code != "" {
			result.EgressError = probeSelectionMessage(code)
			if code == "candidate_not_in_probe_selector" {
				continue
			}
			return nil
		}
		request, err := http.NewRequestWithContext(ctx, http.MethodGet, verification.TraceURL, nil)
		if err != nil {
			result.EgressError = "could not create egress probe request"
			continue
		}
		response, requestErr := httpClient.Do(request)
		var body []byte
		var readErr error
		if requestErr == nil {
			body, readErr = io.ReadAll(io.LimitReader(response.Body, 64<<10))
			response.Body.Close()
		}
		// Keep the response local until its node identity is confirmed again.
		if code := session.confirmCandidate(ctx, result.Name); code != "" {
			result.EgressError = probeSelectionMessage(code)
			return nil
		}
		if requestErr != nil {
			result.EgressError = "egress trace request failed"
			continue
		}
		if readErr != nil || response.StatusCode != http.StatusOK {
			result.EgressError = "egress trace response was not usable"
			continue
		}
		for _, line := range strings.Split(string(body), "\n") {
			key, value, found := strings.Cut(line, "=")
			if found && key == "loc" && len(value) == 2 {
				result.VerifiedRegion = strings.ToUpper(value)
			}
		}
		result.EgressError = ""
		if result.VerifiedRegion == "" {
			result.EgressError = "egress trace did not include a country code"
		}
		copy := *result
		m.publish(scanID, Event{Kind: "egress-verified", Message: "egress verification completed: " + result.Name, At: time.Now().UTC(), Result: &copy})
	}
	return nil
}

// verifyStrict uses only the dedicated selector. Status/body evidence is
// committed after every response has passed a fresh Controller identity check.
func (m *Manager) verifyStrict(ctx context.Context, profile config.ProbeProfile, results []model.NodeResult, scanID string) (warning *model.ScanWarning) {
	if len(profile.StrictProbes) == 0 || len(results) == 0 {
		return nil
	}
	verification := m.currentConfig().Scanner.StrictVerification
	for i := range results {
		results[i].StrictChecks = nil
		results[i].StrictVerificationStatus = "probe_selector_unconfirmed"
		results[i].RestrictionStatus = "not_checked"
	}
	if !verification.Enabled {
		for i := range results {
			results[i].StrictVerificationStatus = "not_configured"
		}
		return nil
	}
	rankNodes(results)
	for i := verification.MaxCandidates; i < len(results); i++ {
		results[i].StrictVerificationStatus = "not_run_limit"
	}
	results = results[:min(len(results), verification.MaxCandidates)]
	proxyURL, err := url.Parse(verification.ProxyURL)
	if err != nil {
		for i := range results {
			results[i].StrictVerificationStatus = "probe_proxy_invalid"
		}
		return nil
	}
	session, code := m.beginProbeSelector(ctx, verification.SelectorGroup)
	if code != "" {
		for i := range results {
			results[i].StrictVerificationStatus = code
		}
		return nil
	}
	defer func() { warning = session.restore("strict") }()
	transport := &http.Transport{
		Proxy: http.ProxyURL(proxyURL), DisableKeepAlives: true,
		DialContext: (&net.Dialer{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS) * time.Millisecond}).DialContext,
	}
	client := &http.Client{Timeout: time.Duration(m.currentConfig().Scanner.TimeoutMS+1000) * time.Millisecond, Transport: transport}
	defer transport.CloseIdleConnections()
	for i := range results {
		result := &results[i]
		if ctx.Err() != nil {
			return nil
		}
		if code := session.selectCandidate(ctx, result.Name); code != "" {
			result.StrictVerificationStatus = code
			if code == "candidate_not_in_probe_selector" {
				continue
			}
			return nil
		}
		checks := make([]model.StrictCheck, 0, len(profile.StrictProbes))
		for _, probe := range profile.StrictProbes {
			if ctx.Err() != nil {
				return nil
			}
			// Every strict request gets a fresh pre-check, even after switch readback.
			if code := session.confirmCandidate(ctx, result.Name); code != "" {
				result.StrictVerificationStatus = code
				return nil
			}
			check := m.runStrictCheck(ctx, client, probe)
			if code := session.confirmCandidate(ctx, result.Name); code != "" {
				result.StrictVerificationStatus = code
				return nil
			}
			checks = append(checks, check)
		}
		result.StrictChecks = checks
		applyStrictOutcome(result)
		copy := *result
		m.publish(scanID, Event{Kind: "strict-verified", Message: "strict service verification completed: " + result.Name, At: time.Now().UTC(), Result: &copy})
	}
	return nil
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
