package model

import "time"

// Monitoring evidence is separate from a scan; failover requires an explicit opt-in.
type MonitorNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
	SeriesID string `json:"series_id,omitempty"`
	Anchor   int64  `json:"anchor,omitempty"`
}

func SameMonitorNode(a, b MonitorNode) bool {
	return a.ID == b.ID && a.Name == b.Name && a.Provider == b.Provider && a.Protocol == b.Protocol
}

type MonitorSeries struct {
	ID          string      `json:"id"`
	Node        MonitorNode `json:"node"`
	ProfileID   string      `json:"profile_id"`
	ProfileHash string      `json:"profile_hash"`
	Anchor      int64       `json:"anchor"`
}

type MonitorRevision struct {
	At   time.Time   `json:"at"`
	Plan MonitorPlan `json:"plan"`
}

type MonitorPlan struct {
	TaskID         string        `json:"task_id"`
	ID             string        `json:"id"`
	Revision       int           `json:"revision"`
	Enabled        bool          `json:"enabled"`
	AutoSwitch     bool          `json:"auto_switch"`
	CandidateLimit int           `json:"candidate_limit"`
	Group          string        `json:"group"`
	ProfileID      string        `json:"profile_id"`
	ProfileHash    string        `json:"profile_hash"`
	Nodes          []MonitorNode `json:"nodes"`
	CreatedAt      time.Time     `json:"created_at"`
	UpdatedAt      time.Time     `json:"updated_at,omitempty"`
}

// A task has a stable identity across plan revisions and evidence epochs.
// Scheduled describes runtime ownership, not whether sampling is enabled.
type MonitorTask struct {
	Plan      MonitorPlan         `json:"plan"`
	Scheduled bool                `json:"scheduled"`
	Runtime   *MonitorTaskSummary `json:"runtime,omitempty"`
}

type MonitorTaskSummary struct {
	Current    string    `json:"current"`
	Issue      string    `json:"issue"`
	Suspended  bool      `json:"suspended"`
	Unhealthy  int       `json:"unhealthy"`
	Unknown    int       `json:"unknown"`
	ObservedAt time.Time `json:"observed_at"`
}

type MonitorRequest struct {
	Revision       int      `json:"revision"`
	Enabled        bool     `json:"enabled"`
	AutoSwitch     *bool    `json:"auto_switch,omitempty"`
	CandidateLimit *int     `json:"candidate_limit,omitempty"`
	Group          string   `json:"group"`
	ProfileID      string   `json:"profile_id"`
	Nodes          []string `json:"nodes"`
}

type MonitorSample struct {
	NodeID      string    `json:"node_id"`
	Kind        string    `json:"kind"` // baseline, current, confirmation, manual
	Slot        int64     `json:"slot"`
	At          time.Time `json:"at"`
	Outcome     string    `json:"outcome"` // success, failure, unknown
	DelayMS     int       `json:"delay_ms"`
	Reason      string    `json:"reason,omitempty"`
	SeriesID    string    `json:"series_id,omitempty"`
	ScheduledAt int64     `json:"scheduled_at,omitempty"`
	ReasonCode  string    `json:"reason_code,omitempty"`
	DataVersion int       `json:"data_version,omitempty"`
	Resolution  string    `json:"resolution,omitempty"`
}

type MonitorState struct {
	Status        string    `json:"status"`
	LastAt        time.Time `json:"last_at"`
	LastSuccess   time.Time `json:"last_success"`
	Failures      int       `json:"failures"`
	Successes     int       `json:"successes"`
	RecoverySlot  int64     `json:"recovery_slot"`
	IncidentStart time.Time `json:"incident_start"`
}

type MonitorEvent struct {
	ID       int64     `json:"id"`
	NodeID   string    `json:"node_id"`
	NodeName string    `json:"node_name"`
	At       time.Time `json:"at"`
	Status   string    `json:"status"`
	Message  string    `json:"message"`
}

type MonitorMetrics struct {
	Score              *float64 `json:"score"`
	Readiness          string   `json:"readiness"`
	Coverage           float64  `json:"coverage"`
	Expected           int      `json:"expected"`
	Samples            int      `json:"samples"`
	SuccessRate        float64  `json:"success_rate"`
	P95MS              int      `json:"p95_ms"`
	Incidents          int      `json:"incidents"`
	FailureSeconds     int      `json:"failure_seconds"`
	ObservedSeconds    int64    `json:"observed_seconds"`
	WindowSeconds      int64    `json:"window_seconds"`
	AvailabilityPoints float64  `json:"availability_points"`
	ContinuityPoints   float64  `json:"continuity_points"`
	LatencyPoints      float64  `json:"latency_points"`
}

type MonitorTrend struct {
	At       time.Time `json:"at"`
	Success  int       `json:"success"`
	Failure  int       `json:"failure"`
	Unknown  int       `json:"unknown"`
	Expected int       `json:"expected"`
	P50      *int      `json:"p50_ms"`
	P95      *int      `json:"p95_ms"`
}

type MonitorTimeline struct {
	Series  MonitorSeries  `json:"series"`
	Active  bool           `json:"active"`
	From    time.Time      `json:"from"`
	To      time.Time      `json:"to"`
	Metrics MonitorMetrics `json:"metrics"`
	Trend   []MonitorTrend `json:"trend"`
}

type MonitorRow struct {
	MonitorNode
	EvidenceVersion string          `json:"evidence_version"`
	State           MonitorState    `json:"state"`
	Metrics         MonitorMetrics  `json:"metrics"`
	Series          []MonitorSample `json:"series"`
}

type MonitorOverview struct {
	Plan            *MonitorPlan   `json:"plan"`
	Current         string         `json:"current"`
	Issue           string         `json:"issue"`
	Suspended       bool           `json:"suspended"`
	FailoverMessage string         `json:"failover_message"`
	ObservedAt      time.Time      `json:"observed_at"`
	Now             time.Time      `json:"now"`
	NextAt          time.Time      `json:"next_at"`
	Rows            []MonitorRow   `json:"rows"`
	Events          []MonitorEvent `json:"events"`
	RetentionDays   int            `json:"retention_days"`
	Window          string         `json:"window"`
	DataVersion     uint64         `json:"data_version"`
	InstanceID      string         `json:"instance_id"`
}
