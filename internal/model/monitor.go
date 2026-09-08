package model

import "time"

// Monitoring evidence is separate from a scan; failover requires an explicit opt-in.
type MonitorNode struct {
	ID       string `json:"id"`
	Name     string `json:"name"`
	Provider string `json:"provider"`
	Protocol string `json:"protocol"`
}

type MonitorPlan struct {
	ID          string        `json:"id"`
	Revision    int           `json:"revision"`
	Enabled     bool          `json:"enabled"`
	AutoSwitch  bool          `json:"auto_switch"`
	Group       string        `json:"group"`
	ProfileID   string        `json:"profile_id"`
	ProfileHash string        `json:"profile_hash"`
	Nodes       []MonitorNode `json:"nodes"`
	CreatedAt   time.Time     `json:"created_at"`
}

type MonitorRequest struct {
	Revision   int      `json:"revision"`
	Enabled    bool     `json:"enabled"`
	AutoSwitch *bool    `json:"auto_switch,omitempty"`
	Group      string   `json:"group"`
	ProfileID  string   `json:"profile_id"`
	Nodes      []string `json:"nodes"`
}

type MonitorSample struct {
	NodeID  string    `json:"node_id"`
	Kind    string    `json:"kind"` // baseline, current, confirmation, manual
	Slot    int64     `json:"slot"`
	At      time.Time `json:"at"`
	Outcome string    `json:"outcome"` // success, failure, unknown
	DelayMS int       `json:"delay_ms"`
	Reason  string    `json:"reason,omitempty"`
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
	Score          *float64 `json:"score"`
	Readiness      string   `json:"readiness"`
	Coverage       float64  `json:"coverage"`
	Expected       int      `json:"expected"`
	Samples        int      `json:"samples"`
	SuccessRate    float64  `json:"success_rate"`
	P95MS          int      `json:"p95_ms"`
	Incidents      int      `json:"incidents"`
	FailureSeconds int      `json:"failure_seconds"`
}

type MonitorRow struct {
	MonitorNode
	State   MonitorState    `json:"state"`
	Metrics MonitorMetrics  `json:"metrics"`
	Series  []MonitorSample `json:"series"`
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
}
