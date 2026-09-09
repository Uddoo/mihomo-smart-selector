package model

import "time"

type MonitorRetention struct {
	Revision      int `json:"revision"`
	RawDays       int `json:"raw_days"`
	AggregateDays int `json:"aggregate_days"`
	EventDays     int `json:"event_days"`
	MaxRawSamples int `json:"max_raw_samples"`
	MaxHourly     int `json:"max_hourly"`
}

type MonitorStorage struct {
	Policy          MonitorRetention `json:"policy"`
	RawSamples      int              `json:"raw_samples"`
	Hourly          int              `json:"hourly"`
	Events          int              `json:"events"`
	PendingHours    int              `json:"pending_hours"`
	UnmappedLegacy  int              `json:"unmapped_legacy"`
	DatabaseBytes   int64            `json:"database_bytes"`
	WALBytes        int64            `json:"wal_bytes"`
	OldestRaw       *time.Time       `json:"oldest_raw"`
	OldestHourly    *time.Time       `json:"oldest_hourly"`
	LastAggregation *time.Time       `json:"last_aggregation"`
}

type MonitorCorrelation struct {
	ID                   int64     `json:"id"`
	PlanID               string    `json:"plan_id"`
	Provider             string    `json:"provider"`
	Status               string    `json:"status"`
	StartedAt            time.Time `json:"started_at"`
	UpdatedAt            time.Time `json:"updated_at"`
	Failed               int       `json:"failed"`
	Comparable           int       `json:"comparable"`
	Monitored            int       `json:"monitored"`
	OtherProviderHealthy bool      `json:"other_provider_healthy"`
	Nodes                []string  `json:"nodes"`
	Message              string    `json:"message"`
	Signature            string    `json:"-"`
	Reliable             bool      `json:"-"`
}

type MonitorActivity struct {
	Key                  string    `json:"key"`
	At                   time.Time `json:"at"`
	Kind                 string    `json:"kind"`
	Status               string    `json:"status"`
	Node                 string    `json:"node,omitempty"`
	SeriesID             string    `json:"series_id,omitempty"`
	Previous             string    `json:"previous,omitempty"`
	Selected             string    `json:"selected,omitempty"`
	RelatedNodes         []string  `json:"related_nodes,omitempty"`
	Failed               int       `json:"failed,omitempty"`
	Comparable           int       `json:"comparable,omitempty"`
	OtherProviderHealthy bool      `json:"other_provider_healthy,omitempty"`
	Group                string    `json:"group,omitempty"`
	Message              string    `json:"message"`
}

type MonitorActivityPage struct {
	Items      []MonitorActivity `json:"items"`
	NextCursor string            `json:"next_cursor,omitempty"`
}
