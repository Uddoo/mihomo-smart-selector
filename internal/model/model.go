package model

import "time"

type ScanStatus string

const (
	ScanPending  ScanStatus = "pending"
	ScanRunning  ScanStatus = "running"
	ScanComplete ScanStatus = "complete"
	ScanFailed   ScanStatus = "failed"
)

type ScanRequest struct {
	TargetGroup string   `json:"target_group"`
	Regions     []string `json:"regions"`
	Providers   []string `json:"providers"`
	Mode        string   `json:"mode"`
}

type ProbeSample struct {
	Probe   string `json:"probe"`
	DelayMS int    `json:"delay_ms,omitempty"`
	Error   string `json:"error,omitempty"`
}

type NodeResult struct {
	Rank           int           `json:"rank"`
	Name           string        `json:"name"`
	Provider       string        `json:"provider,omitempty"`
	InferredRegion string        `json:"inferred_region,omitempty"`
	RegionSource   string        `json:"region_source,omitempty"`
	VerifiedRegion string        `json:"verified_region,omitempty"`
	EgressError    string        `json:"egress_error,omitempty"`
	Samples        []ProbeSample `json:"samples"`
	SuccessRate    float64       `json:"success_rate"`
	P50MS          int           `json:"p50_ms,omitempty"`
	P95MS          int           `json:"p95_ms,omitempty"`
	JitterMS       float64       `json:"jitter_ms,omitempty"`
	Score          float64       `json:"score"`
}

type Scan struct {
	ID          string       `json:"id"`
	Status      ScanStatus   `json:"status"`
	Request     ScanRequest  `json:"request"`
	StartedAt   time.Time    `json:"started_at"`
	CompletedAt *time.Time   `json:"completed_at,omitempty"`
	Error       string       `json:"error,omitempty"`
	Results     []NodeResult `json:"results,omitempty"`
}

type SwitchEvent struct {
	ID        int64     `json:"id"`
	ScanID    string    `json:"scan_id"`
	Group     string    `json:"group"`
	Previous  string    `json:"previous,omitempty"`
	Selected  string    `json:"selected"`
	Reason    string    `json:"reason"`
	CreatedAt time.Time `json:"created_at"`
}
