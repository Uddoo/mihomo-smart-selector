package monitor

// CandidateLimits is also returned by discovery so the editor and scheduler
// share the same bounds. The limit counts nodes, not node/target combinations.
type CandidateLimits struct {
	DefaultCandidateLimit     int `json:"default_candidate_limit"`
	MaxCandidateLimit         int `json:"max_candidate_limit"`
	MaxProbeCount             int `json:"max_probe_count"`
	MinRequestsPerMinute      int `json:"min_requests_per_minute"`
	RequestsPerCandidateProbe int `json:"requests_per_candidate_probe"`
}

func Limits() CandidateLimits {
	return CandidateLimits{DefaultCandidateLimit: 6, MaxCandidateLimit: 30, MaxProbeCount: 6, MinRequestsPerMinute: 12, RequestsPerCandidateProbe: 2}
}

// Budget includes baselines, current-node checks, manual retests, confirmations
// and failover verification. It scales with selected work, not unused capacity.
func requestBudget(nodes, probes int) int {
	l := Limits()
	return max(l.MinRequestsPerMinute, min(nodes, l.MaxCandidateLimit)*min(max(1, probes), l.MaxProbeCount)*l.RequestsPerCandidateProbe)
}
