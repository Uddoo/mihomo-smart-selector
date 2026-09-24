package config

import (
	"fmt"
	"gopkg.in/yaml.v3"
	"os"
	"path/filepath"
	"strings"
)

type Config struct {
	HTTP               HTTPConfig        `yaml:"http"`
	Mihomo             MihomoConfig      `yaml:"mihomo"`
	Storage            StorageConfig     `yaml:"storage"`
	Scanner            ScannerConfig     `yaml:"scanner"`
	Regions            []Region          `yaml:"regions"`
	RegionOverrides    map[string]string `yaml:"region_overrides"`
	EgressVerification EgressConfig      `yaml:"egress_verification"`
	AutoSwitch         AutoSwitchConfig  `yaml:"auto_switch"`
}

type HTTPConfig struct {
	Listen                  string   `yaml:"listen"`
	APIToken                string   `yaml:"api_token"`
	APITokenEnv             string   `yaml:"api_token_env"`
	AllowUnauthenticatedLAN bool     `yaml:"allow_unauthenticated_lan"`
	AllowedCIDRs            []string `yaml:"allowed_cidrs"`
}

type MihomoConfig struct {
	Controller            string `yaml:"controller"`
	SecretEnv             string `yaml:"secret_env"`
	SecretFile            string `yaml:"secret_file"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds"`
}

type RetentionPolicy struct {
	ScanDays  int `yaml:"scan_days" json:"scan_days"`
	MaxScans  int `yaml:"max_scans" json:"max_scans"`
	AuditDays int `yaml:"audit_days" json:"audit_days"`
	MaxAudit  int `yaml:"max_audit" json:"max_audit"`
}

type StorageConfig struct {
	Retention RetentionPolicy `yaml:"retention"`
	Path      string          `yaml:"path"`
}

type ScannerConfig struct {
	MaxActiveScans        int                             `yaml:"max_active_scans"`
	RefineTopK            int                             `yaml:"refine_top_k"`
	ResultMaxAgeSeconds   int                             `yaml:"result_max_age_seconds"`
	Concurrency           int                             `yaml:"concurrency"`
	BatchSize             int                             `yaml:"batch_size"`
	MaxTotalCandidates    int                             `yaml:"max_total_candidates"`
	Samples               int                             `yaml:"samples"`
	TimeoutMS             int                             `yaml:"timeout_ms"`
	MinSuccessRate        float64                         `yaml:"min_success_rate"`
	MedianTargetMS        int                             `yaml:"median_target_ms"`
	P95TargetMS           int                             `yaml:"p95_target_ms"`
	JitterTargetMS        int                             `yaml:"jitter_target_ms"`
	Probes                []Probe                         `yaml:"probes"`
	DefaultProbeProfile   string                          `yaml:"default_probe_profile"`
	ProbeProfiles         []ProbeProfile                  `yaml:"probe_profiles"`
	CustomProbeProfiles   []ProbeProfile                  `yaml:"custom_probe_profiles"`
	ProbeProfileOverrides map[string]ProbeProfileOverride `yaml:"probe_profile_overrides"`
	StrictVerification    StrictVerificationConfig        `yaml:"strict_verification"`
}

type Probe struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	ExpectedStatus string `yaml:"expected_status"`
}

// StrictProbe is evaluated by the selector service through a deliberately
// dedicated probe selector. It is never passed to a provider healthcheck,
// because that Mihomo endpoint cannot enforce expected HTTP statuses.
type StrictProbe struct {
	Name                  string `yaml:"name"`
	URL                   string `yaml:"url"`
	ExpectedStatus        string `yaml:"expected_status"`
	BodyContains          string `yaml:"body_contains"`
	RestrictedStatusCodes []int  `yaml:"restricted_status_codes"`
}

type ProbeProfile struct {
	RequireStrict         bool          `yaml:"require_strict"`
	RequireRegion         bool          `yaml:"require_region"`
	ID                    string        `yaml:"id"`
	Label                 string        `yaml:"label"`
	Description           string        `yaml:"description"`
	ExposeTargetAddresses bool          `yaml:"-"`
	GroupNames            []string      `yaml:"group_names"`
	Probes                []Probe       `yaml:"probes"`
	StrictProbes          []StrictProbe `yaml:"strict_probes"`
	ExpectedRegions       []string      `yaml:"expected_regions"`
	TransportScope        string        `yaml:"transport_scope"`
	RequiresConfiguration bool          `yaml:"requires_configuration"`
	SetupHint             string        `yaml:"setup_hint"`
}

// ProbeProfileOverride changes one built-in profile without replacing every
// other profile. Pointer fields preserve the distinction between an omitted
// value and an intentional false value (notably Emby's setup requirement).
type ProbeProfileOverride struct {
	RequireStrict         *bool         `yaml:"require_strict"`
	RequireRegion         *bool         `yaml:"require_region"`
	Label                 string        `yaml:"label"`
	Description           string        `yaml:"description"`
	Probes                []Probe       `yaml:"probes"`
	StrictProbes          []StrictProbe `yaml:"strict_probes"`
	ExpectedRegions       []string      `yaml:"expected_regions"`
	TransportScope        string        `yaml:"transport_scope"`
	RequiresConfiguration *bool         `yaml:"requires_configuration"`
	SetupHint             string        `yaml:"setup_hint"`
}

type StrictVerificationConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	SelectorGroup string `yaml:"selector_group" json:"selector_group"`
	ProxyURL      string `yaml:"proxy_url" json:"proxy_url"`
	MaxCandidates int    `yaml:"max_candidates" json:"max_candidates"`
}

type Region struct {
	Code    string   `yaml:"code" json:"code"`
	Name    string   `yaml:"name" json:"name"`
	Emoji   string   `yaml:"emoji" json:"emoji"`
	Aliases []string `yaml:"aliases" json:"aliases"`
}

type EgressConfig struct {
	Enabled       bool   `yaml:"enabled" json:"enabled"`
	SelectorGroup string `yaml:"selector_group" json:"selector_group"`
	ProxyURL      string `yaml:"proxy_url" json:"proxy_url"`
	TraceURL      string `yaml:"trace_url" json:"trace_url"`
}

type AutoSwitchConfig struct {
	Enabled                     bool    `yaml:"enabled"`
	MinScore                    float64 `yaml:"min_score"`
	ImprovementThresholdPercent float64 `yaml:"improvement_threshold_percent"`
	BetterRounds                int     `yaml:"better_rounds"`
	CooldownMinutes             int     `yaml:"cooldown_minutes"`
	FailureThreshold            int     `yaml:"failure_threshold"`
}

func Defaults() Config {
	return Config{
		HTTP: HTTPConfig{Listen: "127.0.0.1:8788"},
		Mihomo: MihomoConfig{
			Controller: "http://127.0.0.1:9090", SecretEnv: "MIHOMO_SECRET", RequestTimeoutSeconds: 8,
		},
		Storage: StorageConfig{Path: "data/selector.db", Retention: RetentionPolicy{ScanDays: 30, MaxScans: 200, AuditDays: 180, MaxAudit: 1000}},
		Scanner: ScannerConfig{
			MaxActiveScans: 2,
			RefineTopK:     10, ResultMaxAgeSeconds: 600,
			Concurrency: 4, BatchSize: 60, MaxTotalCandidates: 500, Samples: 3, TimeoutMS: 5000, MinSuccessRate: 0.95,
			MedianTargetMS: 300, P95TargetMS: 800, JitterTargetMS: 200,
			Probes: []Probe{
				{Name: "chatgpt-trace", URL: "https://chatgpt.com/cdn-cgi/trace", ExpectedStatus: "200"},
				{Name: "openai-api-reachability", URL: "https://api.openai.com/v1/models", ExpectedStatus: "401"},
			},
			DefaultProbeProfile: "internet-baseline",
			ProbeProfiles:       defaultProbeProfiles(),
			StrictVerification:  StrictVerificationConfig{MaxCandidates: 60},
		},
		RegionOverrides: map[string]string{},
	}
}

func Load(path string) (Config, error) {
	config := Defaults()
	bytes, err := os.ReadFile(path)
	if err != nil {
		return Config{}, fmt.Errorf("read config: %w", err)
	}
	if err := yaml.Unmarshal(bytes, &config); err != nil {
		return Config{}, fmt.Errorf("parse config: %w", err)
	}
	// Add custom templates without replacing the built-in catalog.
	config.Scanner.ProbeProfiles = append(config.Scanner.ProbeProfiles, config.Scanner.CustomProbeProfiles...)
	if err := config.applyProbeProfileOverrides(); err != nil {
		return Config{}, err
	}
	if strings.TrimSpace(config.HTTP.APIToken) == "" && strings.TrimSpace(config.HTTP.APITokenEnv) != "" {
		config.HTTP.APIToken = os.Getenv(config.HTTP.APITokenEnv)
	}
	base := filepath.Dir(path)
	if config.Mihomo.SecretFile != "" && !filepath.IsAbs(config.Mihomo.SecretFile) {
		config.Mihomo.SecretFile = filepath.Join(base, config.Mihomo.SecretFile)
	}
	if !filepath.IsAbs(config.Storage.Path) {
		config.Storage.Path = filepath.Join(base, config.Storage.Path)
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	config.Regions = MergeRegions(config.Regions)
	return config, nil
}
