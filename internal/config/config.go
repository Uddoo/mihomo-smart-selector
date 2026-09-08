package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"unicode"

	"gopkg.in/yaml.v3"
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

type StorageConfig struct {
	Path string `yaml:"path"`
}

type ScannerConfig struct {
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
		Storage: StorageConfig{Path: "data/selector.db"},
		Scanner: ScannerConfig{
			RefineTopK: 10, ResultMaxAgeSeconds: 600,
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
	return config, nil
}

func (c *Config) applyProbeProfileOverrides() error {
	for id, override := range c.Scanner.ProbeProfileOverrides {
		found := false
		for index := range c.Scanner.ProbeProfiles {
			profile := &c.Scanner.ProbeProfiles[index]
			if !strings.EqualFold(profile.ID, strings.TrimSpace(id)) {
				continue
			}
			found = true
			if strings.TrimSpace(override.Label) != "" {
				profile.Label = override.Label
			}
			if strings.TrimSpace(override.Description) != "" {
				profile.Description = override.Description
			}
			if len(override.Probes) > 0 {
				profile.Probes = append([]Probe(nil), override.Probes...)
			}
			if len(override.StrictProbes) > 0 {
				profile.StrictProbes = append([]StrictProbe(nil), override.StrictProbes...)
			}
			if override.RequireStrict != nil {
				profile.RequireStrict = *override.RequireStrict
			}
			if override.RequireRegion != nil {
				profile.RequireRegion = *override.RequireRegion
			}
			if len(override.ExpectedRegions) > 0 {
				profile.ExpectedRegions = append([]string(nil), override.ExpectedRegions...)
			}
			if strings.TrimSpace(override.TransportScope) != "" {
				profile.TransportScope = override.TransportScope
			}
			if override.RequiresConfiguration != nil {
				profile.RequiresConfiguration = *override.RequiresConfiguration
			}
			if strings.TrimSpace(override.SetupHint) != "" || override.RequiresConfiguration != nil && !*override.RequiresConfiguration {
				profile.SetupHint = override.SetupHint
			}
			break
		}
		if !found {
			return fmt.Errorf("probe_profile_overrides references unknown profile %q", id)
		}
	}
	return nil
}

func (c Config) Validate() error {
	host, _, err := net.SplitHostPort(c.HTTP.Listen)
	if err != nil || host == "" {
		return fmt.Errorf("http.listen must be host:port: %q", c.HTTP.Listen)
	}
	loopback := host == "localhost"
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		loopback = ip.IsLoopback()
	}
	if !loopback && !c.HTTP.AllowUnauthenticatedLAN && strings.TrimSpace(c.HTTP.APIToken) == "" {
		return fmt.Errorf("http.api_token is required when http.listen is not loopback")
	}
	if !loopback && len(c.HTTP.AllowedCIDRs) == 0 {
		return fmt.Errorf("http.allowed_cidrs is required when http.listen is not loopback")
	}
	for _, value := range c.HTTP.AllowedCIDRs {
		if _, err := netip.ParsePrefix(value); err != nil {
			return fmt.Errorf("invalid http.allowed_cidrs entry %q: %w", value, err)
		}
	}
	controller, err := url.ParseRequestURI(c.Mihomo.Controller)
	if err != nil || controller.Scheme == "" || controller.Host == "" {
		return fmt.Errorf("mihomo.controller must be an absolute URL")
	}
	if c.Mihomo.RequestTimeoutSeconds <= 0 {
		return fmt.Errorf("mihomo.request_timeout_seconds must be positive")
	}
	if strings.TrimSpace(c.Mihomo.SecretEnv) == "" {
		return fmt.Errorf("mihomo.secret_env must not be empty")
	}
	if c.Storage.Path == "" {
		return fmt.Errorf("storage.path must not be empty")
	}
	if c.Scanner.Concurrency < 1 || c.Scanner.Concurrency > 16 {
		return fmt.Errorf("scanner.concurrency must be in 1..16")
	}
	if c.Scanner.BatchSize < 1 || c.Scanner.BatchSize > 100 {
		return fmt.Errorf("scanner.batch_size must be in 1..100")
	}
	if c.Scanner.MaxTotalCandidates < 1 || c.Scanner.MaxTotalCandidates > 1000 {
		return fmt.Errorf("scanner.max_total_candidates must be in 1..1000")
	}
	if c.Scanner.RefineTopK < 1 || c.Scanner.RefineTopK > 1000 {
		return fmt.Errorf("scanner.refine_top_k must be in 1..1000")
	}
	if c.Scanner.ResultMaxAgeSeconds < 30 || c.Scanner.ResultMaxAgeSeconds > 86400 {
		return fmt.Errorf("scanner.result_max_age_seconds must be in 30..86400")
	}
	if c.Scanner.Samples < 1 || c.Scanner.Samples > 10 {
		return fmt.Errorf("scanner.samples must be in 1..10")
	}
	if c.Scanner.TimeoutMS < 100 || c.Scanner.TimeoutMS > 60000 {
		return fmt.Errorf("scanner.timeout_ms must be in 100..60000")
	}
	if c.Scanner.MinSuccessRate < 0 || c.Scanner.MinSuccessRate > 1 {
		return fmt.Errorf("scanner.min_success_rate must be in 0..1")
	}
	if c.Scanner.MedianTargetMS <= 0 || c.Scanner.P95TargetMS <= 0 || c.Scanner.JitterTargetMS <= 0 {
		return fmt.Errorf("scanner score targets must be positive")
	}
	if len(c.Scanner.Probes) == 0 && len(c.Scanner.ProbeProfiles) == 0 {
		return fmt.Errorf("scanner requires scanner.probes or scanner.probe_profiles")
	}
	if err := validateProbes("scanner.probes", c.Scanner.Probes); err != nil {
		return err
	}
	if err := c.validateProbeProfiles(); err != nil {
		return err
	}
	if err := c.validateStrictVerification(); err != nil {
		return err
	}
	regionCodes := map[string]bool{}
	for _, region := range c.Regions {
		if region.Code == "" || region.Name == "" || len(region.Aliases) == 0 {
			return fmt.Errorf("every region needs code, name, and at least one alias")
		}
		if regionCodes[region.Code] {
			return fmt.Errorf("duplicate region code %q", region.Code)
		}
		regionCodes[region.Code] = true
	}
	for name, code := range c.RegionOverrides {
		if strings.TrimSpace(name) == "" || !regionCodes[code] {
			return fmt.Errorf("region override %q references unknown region %q", name, code)
		}
	}
	if c.EgressVerification.Enabled {
		if c.EgressVerification.SelectorGroup == "" || c.EgressVerification.ProxyURL == "" || c.EgressVerification.TraceURL == "" {
			return fmt.Errorf("enabled egress_verification requires selector_group, proxy_url, and trace_url")
		}
		if _, err := url.ParseRequestURI(c.EgressVerification.ProxyURL); err != nil {
			return fmt.Errorf("invalid egress_verification.proxy_url: %w", err)
		}
		if _, err := url.ParseRequestURI(c.EgressVerification.TraceURL); err != nil {
			return fmt.Errorf("invalid egress_verification.trace_url: %w", err)
		}
	}
	return nil
}

func (c Config) validateProbeProfiles() error {
	if len(c.Scanner.ProbeProfiles) == 0 {
		return nil
	}
	profileIDs := map[string]bool{}
	groupOwners := map[string]string{}
	for _, profile := range c.Scanner.ProbeProfiles {
		profile.ID = strings.TrimSpace(profile.ID)
		if profile.ID == "" || strings.TrimSpace(profile.Label) == "" {
			return fmt.Errorf("every scanner probe profile needs id and label")
		}
		key := strings.ToLower(profile.ID)
		if profileIDs[key] {
			return fmt.Errorf("duplicate scanner probe profile id %q", profile.ID)
		}
		profileIDs[key] = true
		if profile.TransportScope == "" {
			return fmt.Errorf("probe profile %q needs transport_scope", profile.ID)
		}
		if profile.RequiresConfiguration {
			if strings.TrimSpace(profile.SetupHint) == "" {
				return fmt.Errorf("probe profile %q requires setup_hint when configuration is required", profile.ID)
			}
		} else if len(profile.Probes) == 0 {
			return fmt.Errorf("probe profile %q must contain at least one probe", profile.ID)
		}
		if err := validateProbes("probe profile "+profile.ID, profile.Probes); err != nil {
			return err
		}
		if err := validateStrictProbes(profile); err != nil {
			return err
		}
		if profile.RequireStrict && len(profile.StrictProbes) == 0 {
			return fmt.Errorf("profile %q requires strict probes for require_strict", profile.ID)
		}
		if profile.RequireRegion && len(profile.ExpectedRegions) == 0 {
			return fmt.Errorf("profile %q requires expected_regions for require_region", profile.ID)
		}
		for _, group := range profile.GroupNames {
			groupKey := normaliseGroupName(group)
			if groupKey == "" {
				return fmt.Errorf("probe profile %q has an empty group name", profile.ID)
			}
			if owner, exists := groupOwners[groupKey]; exists {
				return fmt.Errorf("group %q belongs to both probe profiles %q and %q", group, owner, profile.ID)
			}
			groupOwners[groupKey] = profile.ID
		}
		for _, region := range profile.ExpectedRegions {
			if strings.TrimSpace(region) == "" {
				return fmt.Errorf("probe profile %q has an empty expected region", profile.ID)
			}
		}
	}
	if !profileIDs[strings.ToLower(strings.TrimSpace(c.Scanner.DefaultProbeProfile))] {
		return fmt.Errorf("scanner.default_probe_profile %q is not defined", c.Scanner.DefaultProbeProfile)
	}
	return nil
}

func (c Config) validateStrictVerification() error {
	verification := c.Scanner.StrictVerification
	if !verification.Enabled {
		return nil
	}
	if strings.TrimSpace(verification.SelectorGroup) == "" || strings.TrimSpace(verification.ProxyURL) == "" {
		return fmt.Errorf("enabled scanner.strict_verification requires selector_group and proxy_url")
	}
	if verification.MaxCandidates < 1 || verification.MaxCandidates > c.Scanner.MaxTotalCandidates {
		return fmt.Errorf("scanner.strict_verification.max_candidates must be in 1..scanner.max_total_candidates")
	}
	proxyURL, err := url.ParseRequestURI(verification.ProxyURL)
	if err != nil || (proxyURL.Scheme != "http" && proxyURL.Scheme != "https") || proxyURL.Host == "" {
		return fmt.Errorf("scanner.strict_verification.proxy_url must be an absolute HTTP(S) URL")
	}
	return nil
}

func validateProbes(scope string, probes []Probe) error {
	probeNames := map[string]bool{}
	for _, probe := range probes {
		if strings.TrimSpace(probe.Name) == "" || strings.TrimSpace(probe.ExpectedStatus) == "" {
			return fmt.Errorf("every %s probe needs name and expected_status", scope)
		}
		if probeNames[probe.Name] {
			return fmt.Errorf("duplicate %s probe name %q", scope, probe.Name)
		}
		probeNames[probe.Name] = true
		if err := validateHTTPSURL(probe.URL); err != nil {
			return fmt.Errorf("probe %q in %s: %w", probe.Name, scope, err)
		}
		if _, err := validHTTPStatus(probe.ExpectedStatus); err != nil {
			return fmt.Errorf("probe %q in %s: %w", probe.Name, scope, err)
		}
	}
	return nil
}

func validateStrictProbes(profile ProbeProfile) error {
	names := map[string]bool{}
	for _, probe := range profile.StrictProbes {
		if strings.TrimSpace(probe.Name) == "" || strings.TrimSpace(probe.ExpectedStatus) == "" {
			return fmt.Errorf("every strict probe in profile %q needs name and expected_status", profile.ID)
		}
		if names[probe.Name] {
			return fmt.Errorf("duplicate strict probe name %q in profile %q", probe.Name, profile.ID)
		}
		names[probe.Name] = true
		if err := validateHTTPSURL(probe.URL); err != nil {
			return fmt.Errorf("strict probe %q in profile %q: %w", probe.Name, profile.ID, err)
		}
		if _, err := validHTTPStatus(probe.ExpectedStatus); err != nil {
			return fmt.Errorf("strict probe %q in profile %q: %w", probe.Name, profile.ID, err)
		}
		for _, status := range probe.RestrictedStatusCodes {
			if status < 100 || status > 599 {
				return fmt.Errorf("strict probe %q in profile %q has invalid restricted HTTP status %d", probe.Name, profile.ID, status)
			}
		}
	}
	return nil
}

func validateHTTPSURL(value string) error {
	u, err := url.ParseRequestURI(value)
	if err != nil || u.Scheme != "https" || u.Host == "" {
		return fmt.Errorf("must use an absolute HTTPS URL")
	}
	return nil
}

func validHTTPStatus(value string) (int, error) {
	status, err := strconv.Atoi(strings.TrimSpace(value))
	if err != nil || status < 100 || status > 599 {
		return 0, fmt.Errorf("expected_status must be an HTTP status in 100..599")
	}
	return status, nil
}

// ResolveProbeProfile maps the user-facing selector name to a semantic probe
// profile. Selector decorations (emoji, spaces, '+' and punctuation) are
// ignored so an OpenClash label such as "🤖 ChatGPT" maps to "ChatGPT".
func (c Config) ResolveProbeProfile(groupName string) (ProbeProfile, error) {
	if len(c.Scanner.ProbeProfiles) == 0 {
		return ProbeProfile{ID: "legacy", Label: "Legacy global probes", Description: "Uses scanner.probes for every selector.", Probes: append([]Probe(nil), c.Scanner.Probes...), TransportScope: "HTTP latency only"}, nil
	}
	key := normaliseGroupName(groupName)
	for _, profile := range c.Scanner.ProbeProfiles {
		for _, group := range profile.GroupNames {
			if normaliseGroupName(group) == key {
				return cloneProbeProfile(profile), nil
			}
		}
	}
	defaultID := strings.TrimSpace(c.Scanner.DefaultProbeProfile)
	for _, profile := range c.Scanner.ProbeProfiles {
		if strings.EqualFold(profile.ID, defaultID) {
			return cloneProbeProfile(profile), nil
		}
	}
	return ProbeProfile{}, fmt.Errorf("no probe profile is configured for selector %q", groupName)
}

// ProbeProfileByID selects a service independently of the user's group naming.
func (c Config) ProbeProfileByID(id string) (ProbeProfile, error) {
	if id == "legacy" && len(c.Scanner.ProbeProfiles) == 0 {
		return c.ResolveProbeProfile("")
	}
	for _, profile := range c.Scanner.ProbeProfiles {
		if strings.EqualFold(strings.TrimSpace(id), strings.TrimSpace(profile.ID)) {
			return cloneProbeProfile(profile), nil
		}
	}
	return ProbeProfile{}, fmt.Errorf("unknown service profile %q; select an available service", id)
}

func (c Config) SuggestedProfile(groupName string) string {
	for _, profile := range c.Scanner.ProbeProfiles {
		for _, name := range profile.GroupNames {
			if normaliseGroupName(name) == normaliseGroupName(groupName) {
				return profile.ID
			}
		}
	}
	return ""
}

func normaliseGroupName(value string) string {
	var builder strings.Builder
	for _, character := range strings.ToLower(strings.TrimSpace(value)) {
		if unicode.IsLetter(character) || unicode.IsDigit(character) {
			builder.WriteRune(character)
		}
	}
	return builder.String()
}

func cloneProbeProfile(input ProbeProfile) ProbeProfile {
	output := input
	output.GroupNames = append([]string(nil), input.GroupNames...)
	output.Probes = append([]Probe(nil), input.Probes...)
	output.StrictProbes = append([]StrictProbe(nil), input.StrictProbes...)
	output.ExpectedRegions = append([]string(nil), input.ExpectedRegions...)
	return output
}
