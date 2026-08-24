package config

import (
	"fmt"
	"net"
	"net/netip"
	"net/url"
	"os"
	"path/filepath"
	"strings"

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
	Listen       string   `yaml:"listen"`
	APIToken     string   `yaml:"api_token"`
	APITokenEnv  string   `yaml:"api_token_env"`
	AllowedCIDRs []string `yaml:"allowed_cidrs"`
}

type MihomoConfig struct {
	Controller            string `yaml:"controller"`
	SecretEnv             string `yaml:"secret_env"`
	RequestTimeoutSeconds int    `yaml:"request_timeout_seconds"`
}

type StorageConfig struct {
	Path string `yaml:"path"`
}

type ScannerConfig struct {
	Concurrency    int     `yaml:"concurrency"`
	MaxCandidates  int     `yaml:"max_candidates"`
	Samples        int     `yaml:"samples"`
	TimeoutMS      int     `yaml:"timeout_ms"`
	MinSuccessRate float64 `yaml:"min_success_rate"`
	MedianTargetMS int     `yaml:"median_target_ms"`
	P95TargetMS    int     `yaml:"p95_target_ms"`
	JitterTargetMS int     `yaml:"jitter_target_ms"`
	Probes         []Probe `yaml:"probes"`
}

type Probe struct {
	Name           string `yaml:"name"`
	URL            string `yaml:"url"`
	ExpectedStatus string `yaml:"expected_status"`
}

type Region struct {
	Code    string   `yaml:"code" json:"code"`
	Name    string   `yaml:"name" json:"name"`
	Emoji   string   `yaml:"emoji" json:"emoji"`
	Aliases []string `yaml:"aliases" json:"aliases"`
}

type EgressConfig struct {
	Enabled       bool   `yaml:"enabled"`
	SelectorGroup string `yaml:"selector_group"`
	ProxyURL      string `yaml:"proxy_url"`
	TraceURL      string `yaml:"trace_url"`
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
			Concurrency: 4, MaxCandidates: 60, Samples: 3, TimeoutMS: 5000, MinSuccessRate: 0.95,
			MedianTargetMS: 300, P95TargetMS: 800, JitterTargetMS: 200,
			Probes: []Probe{
				{Name: "chatgpt-trace", URL: "https://chatgpt.com/cdn-cgi/trace", ExpectedStatus: "200"},
				{Name: "openai-api-reachability", URL: "https://api.openai.com/v1/models", ExpectedStatus: "401"},
			},
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
	if strings.TrimSpace(config.HTTP.APIToken) == "" && strings.TrimSpace(config.HTTP.APITokenEnv) != "" {
		config.HTTP.APIToken = os.Getenv(config.HTTP.APITokenEnv)
	}
	base := filepath.Dir(path)
	if !filepath.IsAbs(config.Storage.Path) {
		config.Storage.Path = filepath.Join(base, config.Storage.Path)
	}
	if err := config.Validate(); err != nil {
		return Config{}, err
	}
	return config, nil
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
	if !loopback && strings.TrimSpace(c.HTTP.APIToken) == "" {
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
	if c.Scanner.MaxCandidates < 1 || c.Scanner.MaxCandidates > 200 {
		return fmt.Errorf("scanner.max_candidates must be in 1..200")
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
	if len(c.Scanner.Probes) == 0 {
		return fmt.Errorf("scanner.probes must contain at least one probe")
	}
	probeNames := map[string]bool{}
	for _, probe := range c.Scanner.Probes {
		if strings.TrimSpace(probe.Name) == "" || strings.TrimSpace(probe.ExpectedStatus) == "" {
			return fmt.Errorf("every scanner probe needs name and expected_status")
		}
		if probeNames[probe.Name] {
			return fmt.Errorf("duplicate scanner probe name %q", probe.Name)
		}
		probeNames[probe.Name] = true
		u, err := url.ParseRequestURI(probe.URL)
		if err != nil || u.Scheme != "https" || u.Host == "" {
			return fmt.Errorf("probe %q must use an absolute HTTPS URL", probe.Name)
		}
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
