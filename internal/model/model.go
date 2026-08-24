package model

import "time"

type ScanStatus string

const (
	ScanPending   ScanStatus = "pending"
	ScanRunning   ScanStatus = "running"
	ScanComplete  ScanStatus = "complete"
	ScanCancelled ScanStatus = "cancelled"
	ScanFailed    ScanStatus = "failed"
)

type ScanRequest struct {
	TargetGroup string   `json:"target_group"`
	Regions     []string `json:"regions"`
	Providers   []string `json:"providers"`
	Mode        string   `json:"mode"`
}

type ScanPreview struct {
	CandidateCount int `json:"candidate_count"`
	BatchSize      int `json:"batch_size"`
	BatchCount     int `json:"batch_count"`
	SamplesPerNode int `json:"samples_per_node"`
	ProbeRequests  int `json:"probe_requests"`
}

type ScanProgress struct {
	Completed                 int  `json:"completed"`
	Total                     int  `json:"total"`
	CurrentBatch              int  `json:"current_batch"`
	TotalBatches              int  `json:"total_batches"`
	BatchCompleted            int  `json:"batch_completed"`
	BatchTotal                int  `json:"batch_total"`
	Succeeded                 int  `json:"succeeded"`
	Failed                    int  `json:"failed"`
	ElapsedSeconds            int  `json:"elapsed_seconds"`
	EstimatedRemainingSeconds int  `json:"estimated_remaining_seconds"`
	StopAfterCurrentBatch     bool `json:"stop_after_current_batch"`
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

type NodeSummary struct {
	Name           string `json:"name"`
	Provider       string `json:"provider,omitempty"`
	Protocol       string `json:"protocol,omitempty"`
	InferredRegion string `json:"inferred_region,omitempty"`
	RegionSource   string `json:"region_source"`
}

type Scan struct {
	ID          string       `json:"id"`
	Status      ScanStatus   `json:"status"`
	Request     ScanRequest  `json:"request"`
	Progress    ScanProgress `json:"progress"`
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
