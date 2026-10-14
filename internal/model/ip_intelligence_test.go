package model

import "testing"

func TestEgressSnapshotDoesNotAliasProviderEvidence(t *testing.T) {
	score, confidence, reports, flag := 80, 0, 0, false
	input := MonitorEgress{Intelligence: &IPIntelligence{TrustScore: &score}, IntelligenceSources: []IPIntelligence{{Provider: "ipqualityscore", TrustScore: &score, VPN: &flag}, {Provider: "abuseipdb", AbuseConfidenceScore: &confidence, TotalReports: &reports}}}
	copy := input.Clone()
	*copy.IntelligenceSources[0].TrustScore = 1
	*copy.IntelligenceSources[0].VPN = true
	*copy.IntelligenceSources[1].AbuseConfidenceScore = 99
	copy.IntelligenceSources[0].Source = "changed"
	if *input.IntelligenceSources[0].TrustScore != 80 || *input.IntelligenceSources[0].VPN || *input.IntelligenceSources[1].AbuseConfidenceScore != 0 || input.IntelligenceSources[0].Source != "" || *copy.Intelligence.TrustScore != 80 {
		t.Fatal("provider or legacy snapshot alias")
	}
}
