package config

import (
	"errors"
	"os"
	"path/filepath"
	"regexp"
	"strings"
)

// IPIntelligenceConfig keeps credentials outside runtime settings and API responses.
// A nonempty environment value takes precedence over the optional secret file.
type IPIntelligenceConfig struct {
	// Legacy fields remain an IPQualityScore fallback for existing installations.
	APIKeyEnv      string             `yaml:"api_key_env"`
	APIKeyFile     string             `yaml:"api_key_file"`
	IPQualityScore IPCredentialConfig `yaml:"ipqualityscore"`
	Scamalytics    ScamalyticsConfig  `yaml:"scamalytics"`
	IPAPIIS        IPCredentialConfig `yaml:"ipapi_is"`
	AbuseIPDB      IPCredentialConfig `yaml:"abuseipdb"`
}

type IPCredentialConfig struct {
	RuntimeKey *string `yaml:"-" json:"-"`
	Enabled    *bool   `yaml:"enabled"`
	APIKeyEnv  string  `yaml:"api_key_env"`
	APIKeyFile string  `yaml:"api_key_file"`
}

type ScamalyticsConfig struct {
	IPCredentialConfig `yaml:",inline"`
	Username           string `yaml:"username"`
	UsernameEnv        string `yaml:"username_env"`
	Server             string `yaml:"server"` // eu (api12) or us (api11), matching the account.
}

type IPProviderConfig struct {
	ID          string
	Credentials IPCredentialConfig
	Username    string
	Server      string
}

func (c IPIntelligenceConfig) Providers() []IPProviderConfig {
	ipqs := c.IPQualityScore
	if ipqs.APIKeyEnv == "" && ipqs.APIKeyFile == "" {
		ipqs.APIKeyEnv, ipqs.APIKeyFile = c.APIKeyEnv, c.APIKeyFile
	}
	username := strings.TrimSpace(c.Scamalytics.Username)
	if username == "" {
		username = strings.TrimSpace(os.Getenv(c.Scamalytics.UsernameEnv))
	}
	server := strings.ToLower(strings.TrimSpace(c.Scamalytics.Server))
	if server == "" {
		server = "eu"
	}
	return []IPProviderConfig{
		{ID: "ipqualityscore", Credentials: ipqs},
		{ID: "scamalytics", Credentials: c.Scamalytics.IPCredentialConfig, Username: username, Server: server},
		{ID: "ipapi_is", Credentials: c.IPAPIIS},
		{ID: "abuseipdb", Credentials: c.AbuseIPDB},
	}
}

func (c *IPIntelligenceConfig) ResolvePaths(base string) {
	for _, path := range []*string{&c.APIKeyFile, &c.IPQualityScore.APIKeyFile, &c.Scamalytics.APIKeyFile, &c.IPAPIIS.APIKeyFile, &c.AbuseIPDB.APIKeyFile} {
		if *path != "" && !filepath.IsAbs(*path) {
			*path = filepath.Join(base, *path)
		}
	}
}

var scamalyticsUsername = regexp.MustCompile(`^[A-Za-z0-9_.-]{1,128}$`)

func (c IPIntelligenceConfig) Validate() error {
	server := strings.ToLower(strings.TrimSpace(c.Scamalytics.Server))
	if server != "" && server != "eu" && server != "us" {
		return errors.New("Scamalytics API 节点必须为 eu 或 us")
	}
	if c.Scamalytics.Username != "" && !scamalyticsUsername.MatchString(c.Scamalytics.Username) {
		return errors.New("Scamalytics 用户名格式无效")
	}
	return nil
}

func (c IPIntelligenceConfig) APIKey() (string, error) {
	return (IPCredentialConfig{APIKeyEnv: c.APIKeyEnv, APIKeyFile: c.APIKeyFile}).APIKey()
}

func (c IPCredentialConfig) APIKey() (string, error) {
	if c.RuntimeKey != nil {
		return *c.RuntimeKey, nil
	}
	key := strings.TrimSpace(os.Getenv(c.APIKeyEnv))
	if key == "" && c.APIKeyFile != "" {
		data, err := os.ReadFile(c.APIKeyFile)
		if err != nil {
			return "", errors.New("无法读取 IP 情报密钥文件")
		}
		if len(data) > 4096 {
			return "", errors.New("IP 情报密钥格式无效")
		}
		key = strings.TrimSpace(string(data))
	}
	if len(key) > 4096 || strings.ContainsAny(key, "\r\n") {
		return "", errors.New("IP 情报密钥格式无效")
	}
	return key, nil
}
