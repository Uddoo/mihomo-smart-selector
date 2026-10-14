package model

import "time"

// These observations never participate in HTTPS health scores or failover.
// Optional flags distinguish an absent provider field from a negative result.
type IPIntelligence struct {
	Provider             string    `json:"provider"`
	ScoreKind            string    `json:"score_kind"` // fraud_risk, abuse_confidence, network_abuse
	Status               string    `json:"status"`
	Source               string    `json:"source"`
	TrustScore           *int      `json:"trust_score"`
	FraudScore           *int      `json:"fraud_score"`
	ConnectionType       string    `json:"connection_type"`
	CountryCode          string    `json:"country_code"`
	Region               string    `json:"region"`
	City                 string    `json:"city"`
	ASN                  int       `json:"asn"`
	ISP                  string    `json:"isp"`
	Organization         string    `json:"organization"`
	VPN                  *bool     `json:"vpn"`
	Proxy                *bool     `json:"proxy"`
	Tor                  *bool     `json:"tor"`
	RecentAbuse          *bool     `json:"recent_abuse"`
	Datacenter           *bool     `json:"datacenter"`
	Abuser               *bool     `json:"abuser"`
	AbuseConfidenceScore *int      `json:"abuse_confidence_score"`
	TotalReports         *int      `json:"total_reports"`
	ReportsWindowDays    int       `json:"reports_window_days"`
	CompanyAbuseScore    string    `json:"company_abuse_score"`
	ASNAbuseScore        string    `json:"asn_abuse_score"`
	FetchedAt            time.Time `json:"fetched_at"`
	ExpiresAt            time.Time `json:"expires_at"`
	Error                string    `json:"error,omitempty"`
}

type MonitorEgress struct {
	Status              string           `json:"status"`
	IP                  string           `json:"ip"`
	CountryCode         string           `json:"country_code"`
	ObservedAt          time.Time        `json:"observed_at"`
	AttemptedAt         time.Time        `json:"attempted_at"`
	Error               string           `json:"error,omitempty"`
	Intelligence        *IPIntelligence  `json:"intelligence,omitempty"`
	IntelligenceSources []IPIntelligence `json:"intelligence_sources,omitempty"`
}

func copyOptional[T any](value *T) *T {
	if value == nil {
		return nil
	}
	copy := *value
	return &copy
}

func (v IPIntelligence) Clone() IPIntelligence {
	v.TrustScore, v.FraudScore = copyOptional(v.TrustScore), copyOptional(v.FraudScore)
	v.VPN, v.Proxy, v.Tor, v.RecentAbuse = copyOptional(v.VPN), copyOptional(v.Proxy), copyOptional(v.Tor), copyOptional(v.RecentAbuse)
	v.Datacenter, v.Abuser = copyOptional(v.Datacenter), copyOptional(v.Abuser)
	v.AbuseConfidenceScore, v.TotalReports = copyOptional(v.AbuseConfidenceScore), copyOptional(v.TotalReports)
	return v
}

func (v MonitorEgress) Clone() MonitorEgress {
	if v.Intelligence != nil {
		data := v.Intelligence.Clone()
		v.Intelligence = &data
	}
	if v.IntelligenceSources != nil {
		v.IntelligenceSources = append([]IPIntelligence{}, v.IntelligenceSources...)
		for i := range v.IntelligenceSources {
			v.IntelligenceSources[i] = v.IntelligenceSources[i].Clone()
		}
	}
	return v
}
