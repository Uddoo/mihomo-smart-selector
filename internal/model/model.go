package model

import "time"

type ScanStatus string

const (
	ScanInterrupted ScanStatus = "interrupted"
	ScanPending     ScanStatus = "pending"
	ScanRunning     ScanStatus = "running"
	ScanComplete    ScanStatus = "complete"
	ScanCancelled   ScanStatus = "cancelled"
	ScanFailed      ScanStatus = "failed"
)

type ScanRequest struct {
	Nodes       []string `json:"nodes,omitempty"`
	TargetGroup string   `json:"target_group"`
	ProfileID   string   `json:"profile_id,omitempty"`
	Regions     []string `json:"regions"`
	Providers   []string `json:"providers"`
	Mode        string   `json:"mode"`
}

type ServiceBinding struct {
	Group     string `json:"group"`
	ProfileID string `json:"profile_id"`
	Status    string `json:"status,omitempty"`
}

type ServiceCatalog struct {
	Profiles         []ProbeProfileSummary `json:"profiles"`
	Bindings         []ServiceBinding      `json:"bindings"`
	Suggestions      map[string]string     `json:"suggestions"`
	DefaultProfileID string                `json:"default_profile_id"`
}

type ScanPreview struct {
	RefineCandidates int                 `json:"refine_candidates"`
	CandidateCount   int                 `json:"candidate_count"`
	BatchSize        int                 `json:"batch_size"`
	BatchCount       int                 `json:"batch_count"`
	SamplesPerNode   int                 `json:"samples_per_node"`
	ProbeRequests    int                 `json:"probe_requests"`
	Profile          ProbeProfileSummary `json:"profile"`
	Ready            bool                `json:"ready"`
	Reason           string              `json:"reason,omitempty"`
	ReasonCode       string              `json:"reason_code,omitempty"`
	NestedSelectors  []NestedSelector    `json:"nested_selectors,omitempty"`
}

// NestedSelector suggests an explicit change of scan target. Its members are
// never flattened into the parent selector or selected through the parent.
type NestedSelector struct {
	Group          string   `json:"group"`
	Path           []string `json:"path"`
	CandidateCount int      `json:"candidate_count"`
	OnCurrentPath  bool     `json:"on_current_path"`
}

type ProbeTargetSummary struct {
	Name           string `json:"name"`
	Address        string `json:"address,omitempty"`
	ExpectedStatus string `json:"expected_status"`
	Kind           string `json:"kind"`
	AddressVisible bool   `json:"address_visible"`
}

// ProbeProfileSummary exposes only built-in, public test targets. A custom
// profile can point at a private Emby hostname, which must remain hidden from
// unauthenticated LAN browsers while still describing its test semantics.
type ProbeProfileSummary struct {
	RequireStrict               bool                 `json:"require_strict"`
	RequireRegion               bool                 `json:"require_region"`
	ID                          string               `json:"id"`
	Label                       string               `json:"label"`
	Description                 string               `json:"description"`
	ProbeCount                  int                  `json:"probe_count"`
	StrictProbeCount            int                  `json:"strict_probe_count"`
	StrictVerificationAvailable bool                 `json:"strict_verification_available"`
	RequiresConfiguration       bool                 `json:"requires_configuration"`
	SetupHint                   string               `json:"setup_hint,omitempty"`
	ExpectedRegions             []string             `json:"expected_regions,omitempty"`
	TransportScope              string               `json:"transport_scope"`
	Targets                     []ProbeTargetSummary `json:"targets,omitempty"`
}

type ScanProgress struct {
	Stage                     string `json:"stage,omitempty"`
	Completed                 int    `json:"completed"`
	Total                     int    `json:"total"`
	CurrentBatch              int    `json:"current_batch"`
	TotalBatches              int    `json:"total_batches"`
	BatchCompleted            int    `json:"batch_completed"`
	BatchTotal                int    `json:"batch_total"`
	Succeeded                 int    `json:"succeeded"`
	Failed                    int    `json:"failed"`
	ElapsedSeconds            int    `json:"elapsed_seconds"`
	EstimatedRemainingSeconds int    `json:"estimated_remaining_seconds"`
	StopAfterCurrentBatch     bool   `json:"stop_after_current_batch"`
}

type ProbeSample struct {
	Probe   string `json:"probe"`
	DelayMS int    `json:"delay_ms,omitempty"`
	Error   string `json:"error,omitempty"`
}

type StrictCheck struct {
	Probe          string `json:"probe"`
	ExpectedStatus string `json:"expected_status"`
	ObservedStatus int    `json:"observed_status,omitempty"`
	Status         string `json:"status"`
	BodyMatched    *bool  `json:"body_matched,omitempty"`
	Error          string `json:"error,omitempty"`
}

type ScoreBreakdown struct {
	Reliability float64 `json:"reliability"`
	P50         float64 `json:"p50"`
	P95         float64 `json:"p95"`
	Jitter      float64 `json:"jitter"`
	Region      float64 `json:"region"`
	Total       float64 `json:"total"`
}

type NodeResult struct {
	ScreeningSamples         int            `json:"screening_samples"`
	RefinementSamples        int            `json:"refinement_samples"`
	Stage                    string         `json:"stage,omitempty"`
	MeasuredAt               time.Time      `json:"measured_at,omitzero"`
	ExpiresAt                time.Time      `json:"expires_at,omitzero"`
	SelectionReason          string         `json:"selection_reason,omitempty"`
	Rank                     int            `json:"rank"`
	Name                     string         `json:"name"`
	Provider                 string         `json:"provider,omitempty"`
	InferredRegion           string         `json:"inferred_region,omitempty"`
	RegionSource             string         `json:"region_source,omitempty"`
	VerifiedRegion           string         `json:"verified_region,omitempty"`
	EgressError              string         `json:"egress_error,omitempty"`
	Samples                  []ProbeSample  `json:"samples"`
	StrictChecks             []StrictCheck  `json:"strict_checks,omitempty"`
	SuccessRate              float64        `json:"success_rate"`
	P50MS                    int            `json:"p50_ms,omitempty"`
	P95MS                    int            `json:"p95_ms,omitempty"`
	JitterMS                 float64        `json:"jitter_ms,omitempty"`
	Score                    float64        `json:"score"`
	ScoreBreakdown           ScoreBreakdown `json:"score_breakdown"`
	ReachabilityStatus       string         `json:"reachability_status"`
	StrictVerificationStatus string         `json:"strict_verification_status"`
	RestrictionStatus        string         `json:"restriction_status"`
	RegionVerificationStatus string         `json:"region_verification_status"`
	TransportStatus          string         `json:"transport_status"`
}

type NodeSummary struct {
	Name             string   `json:"name"`
	Provider         string   `json:"provider,omitempty"`
	Protocol         string   `json:"protocol,omitempty"`
	InferredRegion   string   `json:"inferred_region,omitempty"`
	RegionSource     string   `json:"region_source"`
	EntryKind        string   `json:"entry_kind"`
	RegionReason     string   `json:"region_reason,omitempty"`
	RegionCandidates []string `json:"region_candidates,omitempty"`
	RegionEvidence   []string `json:"region_evidence,omitempty"`
}

type Scan struct {
	ControllerScope string              `json:"-"`
	ID              string              `json:"id"`
	Status          ScanStatus          `json:"status"`
	Request         ScanRequest         `json:"request"`
	Profile         ProbeProfileSummary `json:"profile"`
	Progress        ScanProgress        `json:"progress"`
	StartedAt       time.Time           `json:"started_at"`
	CompletedAt     *time.Time          `json:"completed_at,omitempty"`
	Error           string              `json:"error,omitempty"`
	Results         []NodeResult        `json:"results,omitempty"`
}

type SwitchEvent struct {
	ControllerScope string    `json:"-"`
	Status          string    `json:"status"`
	RequestID       string    `json:"request_id,omitempty"`
	AuditPersisted  bool      `json:"audit_persisted"`
	ID              int64     `json:"id"`
	ScanID          string    `json:"scan_id"`
	Group           string    `json:"group"`
	Previous        string    `json:"previous,omitempty"`
	Selected        string    `json:"selected"`
	Reason          string    `json:"reason"`
	CreatedAt       time.Time `json:"created_at"`
}
