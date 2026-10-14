package ipintel

import (
	"encoding/json"
	"errors"
	"io"
	"os"
	"path/filepath"
	"strings"
	"sync"
	"unicode"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

var ErrSettingsConflict = errors.New("IP 情报配置已被其他页面更新，请重新读取后再保存")

type ProviderSettings struct {
	ID            string `json:"id"`
	Name          string `json:"name"`
	Enabled       bool   `json:"enabled"`
	KeyConfigured bool   `json:"key_configured"`
	KeySource     string `json:"key_source"`
	Username      string `json:"username,omitempty"`
	Server        string `json:"server,omitempty"`
	Error         string `json:"error,omitempty"`
}
type SettingsState struct {
	Revision  int                `json:"revision"`
	Providers []ProviderSettings `json:"providers"`
}
type ProviderUpdate struct {
	ID        string `json:"id"`
	Enabled   *bool  `json:"enabled"`
	KeyAction string `json:"key_action"`
	Key       string `json:"key"`
	Username  string `json:"username"`
	Server    string `json:"server"`
}
type SettingsUpdate struct {
	Revision  int              `json:"revision"`
	Providers []ProviderUpdate `json:"providers"`
}
type providerRecord struct {
	Enabled  bool    `json:"enabled"`
	Key      *string `json:"key,omitempty"`
	Username string  `json:"username,omitempty"`
	Server   string  `json:"server,omitempty"`
}
type settingsRecord struct {
	Revision  int                       `json:"revision"`
	Providers map[string]providerRecord `json:"providers"`
}
type Settings struct {
	mu     sync.Mutex
	base   config.IPIntelligenceConfig
	path   string
	record settingsRecord
}

func OpenSettings(base config.IPIntelligenceConfig, path string) (*Settings, error) {
	s := &Settings{base: base, path: path, record: settingsRecord{Providers: map[string]providerRecord{}}}
	f, err := os.Open(path)
	if os.IsNotExist(err) {
		return s, nil
	}
	if err != nil {
		return nil, errors.New("无法读取 IP 情报私有配置")
	}
	defer f.Close()
	data, err := io.ReadAll(io.LimitReader(f, (128<<10)+1))
	if err != nil || len(data) > 128<<10 || !strings.HasPrefix(strings.TrimSpace(string(data)), "{") || json.Unmarshal(data, &s.record) != nil || s.record.Revision < 0 {
		return nil, errors.New("IP 情报私有配置无效")
	}
	if s.record.Providers == nil {
		s.record.Providers = map[string]providerRecord{}
	}
	for id, r := range s.record.Providers {
		validAccount := r.Username == "" && (r.Key == nil || *r.Key == "") && (r.Server == "eu" || r.Server == "us") || validScamalyticsAccount(credentials{username: r.Username, server: r.Server})
		if !providerID(id) || r.Key != nil && validateKey(*r.Key, false) != nil || id == "scamalytics" && !validAccount {
			return nil, errors.New("IP 情报私有配置无效")
		}
	}
	return s, nil
}

func providerID(id string) bool {
	return id == "ipqualityscore" || id == "scamalytics" || id == "ipapi_is" || id == "abuseipdb"
}

func validateKey(key string, required bool) error {
	if len(key) > 4096 || strings.IndexFunc(key, unicode.IsControl) >= 0 || required && strings.TrimSpace(key) == "" {
		return errors.New("API 密钥须为非空单行文本，且不超过 4096 字节")
	}
	return nil
}

func (s *Settings) effective() config.IPIntelligenceConfig {
	c := s.base
	// Resolve the legacy fallback before applying a browser-owned credential.
	c.IPQualityScore = s.base.Providers()[0].Credentials
	for id, r := range s.record.Providers {
		var target *config.IPCredentialConfig
		switch id {
		case "ipqualityscore":
			target = &c.IPQualityScore
		case "scamalytics":
			target = &c.Scamalytics.IPCredentialConfig
			c.Scamalytics.Username = r.Username
			c.Scamalytics.UsernameEnv = ""
			c.Scamalytics.Server = r.Server
		case "ipapi_is":
			target = &c.IPAPIIS
		case "abuseipdb":
			target = &c.AbuseIPDB
		}
		enabled := r.Enabled
		target.Enabled = &enabled
		if r.Key != nil {
			key := *r.Key
			target.RuntimeKey = &key
		}
	}
	return c
}
func (s *Settings) Effective() config.IPIntelligenceConfig {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.effective()
}

func (s *Settings) state() SettingsState {
	v := SettingsState{Revision: s.record.Revision, Providers: []ProviderSettings{}}
	for _, p := range s.effective().Providers() {
		name := map[string]string{"ipqualityscore": "IPQualityScore", "scamalytics": "Scamalytics", "ipapi_is": "ipapi.is", "abuseipdb": "AbuseIPDB"}[p.ID]
		r := ProviderSettings{ID: p.ID, Name: name, Enabled: p.Credentials.Enabled == nil || *p.Credentials.Enabled, KeySource: "server", Username: p.Username, Server: p.Server}
		if p.ID != "scamalytics" {
			r.Username, r.Server = "", ""
		}
		if p.Credentials.RuntimeKey != nil {
			r.KeySource = "custom"
			if *p.Credentials.RuntimeKey == "" {
				r.KeySource = "none"
			}
		}
		key, err := p.Credentials.APIKey()
		r.KeyConfigured = key != "" && err == nil
		if err != nil {
			r.Error = err.Error()
		}
		v.Providers = append(v.Providers, r)
	}
	return v
}
func (s *Settings) State() SettingsState { s.mu.Lock(); defer s.mu.Unlock(); return s.state() }

func (s *Settings) Save(update SettingsUpdate) (SettingsState, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if update.Revision != s.record.Revision {
		return SettingsState{}, ErrSettingsConflict
	}
	if len(update.Providers) < 1 || len(update.Providers) > 4 {
		return SettingsState{}, errors.New("请选择有效的 IP 情报来源")
	}
	next := settingsRecord{Revision: s.record.Revision + 1, Providers: map[string]providerRecord{}}
	for id, r := range s.record.Providers {
		next.Providers[id] = r
	}
	seen := map[string]bool{}
	for _, p := range update.Providers {
		if !providerID(p.ID) || seen[p.ID] || p.Enabled == nil {
			return SettingsState{}, errors.New("IP 情报来源或启用状态无效")
		}
		seen[p.ID] = true
		r := next.Providers[p.ID]
		r.Enabled = *p.Enabled
		if p.KeyAction != "replace" && p.Key != "" {
			return SettingsState{}, errors.New("只有替换操作可以提交 API 密钥")
		}
		switch p.KeyAction {
		case "keep":
		case "replace":
			if err := validateKey(p.Key, true); err != nil {
				return SettingsState{}, err
			}
			key := strings.TrimSpace(p.Key)
			r.Key = &key
		case "clear":
			key := ""
			r.Key = &key
		case "server":
			r.Key = nil
		default:
			return SettingsState{}, errors.New("请选择有效的 API 密钥操作")
		}
		if p.ID == "scamalytics" {
			r.Username = strings.TrimSpace(p.Username)
			r.Server = strings.ToLower(strings.TrimSpace(p.Server))
			if r.Server == "" {
				r.Server = "eu"
			}
			if r.Username == "" {
				if r.Key != nil && *r.Key != "" {
					return SettingsState{}, errors.New("Scamalytics 需要填写帐号用户名")
				}
				if r.Server != "eu" && r.Server != "us" {
					return SettingsState{}, errors.New("Scamalytics API 节点必须为 eu 或 us")
				}
			} else if !validScamalyticsAccount(credentials{username: r.Username, server: r.Server}) {
				return SettingsState{}, errors.New("Scamalytics 用户名或 API 节点无效")
			}
		}
		next.Providers[p.ID] = r
	}
	data, err := json.Marshal(next)
	if err != nil {
		return SettingsState{}, errors.New("无法保存 IP 情报配置")
	}
	if err = writeSettings(s.path, data); err != nil {
		return SettingsState{}, errors.New("无法保存 IP 情报配置，请检查服务端目录权限")
	}
	s.record = next
	return s.state(), nil
}

func writeSettings(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".ip-intelligence-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if err = f.Chmod(0600); err == nil {
		_, err = f.Write(data)
	}
	if err == nil {
		err = f.Sync()
	}
	closeErr := f.Close()
	if err != nil {
		return err
	}
	if closeErr != nil {
		return closeErr
	}
	return os.Rename(f.Name(), path)
}
