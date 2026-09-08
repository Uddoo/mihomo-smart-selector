package config

import (
	"fmt"
	"net/url"
)

// RuntimeSettings contains only user-facing scan controls, never credentials or listeners.
type RuntimeSettings struct {
	MaxActiveScans      int                      `json:"max_active_scans"`
	Retention           RetentionPolicy          `json:"retention"`
	RefineTopK          int                      `json:"refine_top_k"`
	ResultMaxAgeSeconds int                      `json:"result_max_age_seconds"`
	Revision            int                      `json:"revision"`
	Concurrency         int                      `json:"concurrency"`
	BatchSize           int                      `json:"batch_size"`
	MaxCandidates       int                      `json:"max_candidates"`
	Samples             int                      `json:"samples"`
	TimeoutMS           int                      `json:"timeout_ms"`
	MinSuccessRate      float64                  `json:"min_success_rate"`
	MedianTargetMS      int                      `json:"median_target_ms"`
	P95TargetMS         int                      `json:"p95_target_ms"`
	JitterTargetMS      int                      `json:"jitter_target_ms"`
	DefaultProfile      string                   `json:"default_profile"`
	Egress              EgressConfig             `json:"egress"`
	Strict              StrictVerificationConfig `json:"strict"`
}

func (c Config) RuntimeSettings() RuntimeSettings {
	s := c.Scanner
	return RuntimeSettings{MaxActiveScans: s.MaxActiveScans, Retention: c.Storage.Retention, RefineTopK: s.RefineTopK, ResultMaxAgeSeconds: s.ResultMaxAgeSeconds, Concurrency: s.Concurrency, BatchSize: s.BatchSize, MaxCandidates: s.MaxTotalCandidates, Samples: s.Samples, TimeoutMS: s.TimeoutMS, MinSuccessRate: s.MinSuccessRate, MedianTargetMS: s.MedianTargetMS, P95TargetMS: s.P95TargetMS, JitterTargetMS: s.JitterTargetMS, DefaultProfile: s.DefaultProbeProfile, Egress: c.EgressVerification, Strict: s.StrictVerification}
}

func (c Config) WithRuntimeSettings(s RuntimeSettings) (Config, error) {
	c.Scanner.MaxActiveScans = s.MaxActiveScans
	c.Storage.Retention = s.Retention
	c.Scanner.RefineTopK = s.RefineTopK
	c.Scanner.ResultMaxAgeSeconds = s.ResultMaxAgeSeconds
	c.Scanner.Concurrency = s.Concurrency
	c.Scanner.BatchSize = s.BatchSize
	c.Scanner.MaxTotalCandidates = s.MaxCandidates
	c.Scanner.Samples = s.Samples
	c.Scanner.TimeoutMS = s.TimeoutMS
	c.Scanner.MinSuccessRate = s.MinSuccessRate
	c.Scanner.MedianTargetMS = s.MedianTargetMS
	c.Scanner.P95TargetMS = s.P95TargetMS
	c.Scanner.JitterTargetMS = s.JitterTargetMS
	c.Scanner.DefaultProbeProfile = s.DefaultProfile
	c.Scanner.StrictVerification = s.Strict
	c.EgressVerification = s.Egress
	if err := c.Validate(); err != nil {
		return c, err
	}
	for _, value := range []string{s.Egress.ProxyURL, s.Strict.ProxyURL, s.Egress.TraceURL} {
		if value == "" {
			continue
		}
		u, err := url.ParseRequestURI(value)
		if err != nil || (u.Scheme != "http" && u.Scheme != "https") || u.Host == "" || u.User != nil || u.Fragment != "" {
			return c, fmt.Errorf("探测地址必须是无用户名密码的完整 HTTP(S) URL")
		}
	}
	if s.Egress.Enabled {
		if err := validateHTTPSURL(s.Egress.TraceURL); err != nil {
			return c, fmt.Errorf("出口探测地址必须使用 HTTPS")
		}
	}
	return c, nil
}
