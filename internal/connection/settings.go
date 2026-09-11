// Package connection manages the next-start Controller configuration separately
// from running scans and monitoring. Credentials never enter public responses.
package connection

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"sync"
	"time"
	"unicode"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
)

var ErrConflict = errors.New("连接配置已被其他页面更新，请重新加载后再保存")

type PublicSettings struct {
	Controller            string `json:"controller"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	SecretSource          string `json:"secret_source"`
	SecretConfigured      bool   `json:"secret_configured"`
}

type State struct {
	Revision        int            `json:"revision"`
	Override        bool           `json:"override"`
	Active          PublicSettings `json:"active"`
	Saved           PublicSettings `json:"saved"`
	RestartRequired bool           `json:"restart_required"`
}

type Update struct {
	Revision              int    `json:"revision"`
	Controller            string `json:"controller"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds"`
	SecretAction          string `json:"secret_action"`
	Secret                string `json:"secret"`
	UseServer             bool   `json:"use_server"`
}

// This representation is private, stored in a separate owner-readable file.
type record struct {
	Revision              int    `json:"revision"`
	Override              bool   `json:"override"`
	Controller            string `json:"controller,omitempty"`
	RequestTimeoutSeconds int    `json:"request_timeout_seconds,omitempty"`
	SecretSource          string `json:"secret_source,omitempty"`
	Secret                string `json:"secret,omitempty"`
}

type Manager struct {
	mu         sync.Mutex
	base       config.MihomoConfig
	path       string
	saved      record
	active     record
	activeView PublicSettings
}

// Open applies the saved override once at startup. The returned config is also
// used for Controller-scoped history, settings and monitoring bindings.
func Open(base config.MihomoConfig, path string) (*Manager, config.MihomoConfig, *mihomo.HTTPClient, error) {
	m := &Manager{base: base, path: path}
	data, err := os.ReadFile(path)
	if err != nil && !os.IsNotExist(err) {
		return nil, base, nil, fmt.Errorf("无法读取连接配置文件")
	}
	if err == nil && (len(data) == 0 || len(data) > 64<<10) {
		return nil, base, nil, fmt.Errorf("连接配置文件无效")
	}
	if len(data) > 0 {
		if err := json.Unmarshal(data, &m.saved); err != nil {
			return nil, base, nil, fmt.Errorf("连接配置文件无效")
		}
	}
	if m.saved.Override {
		if err := validate(m.saved); err != nil {
			return nil, base, nil, err
		}
	}
	cfg, secret, err := m.resolve(m.saved)
	if err != nil {
		return nil, base, nil, err
	}
	client, err := mihomo.NewWithSecret(cfg, secret)
	if err != nil {
		return nil, base, nil, err
	}
	m.active = m.saved
	m.activeView = m.public(m.active)
	m.activeView.SecretConfigured = secret != ""
	return m, cfg, client, nil
}

func (m *Manager) public(r record) PublicSettings {
	if !r.Override {
		return PublicSettings{m.base.Controller, m.base.RequestTimeoutSeconds, "server", m.base.SecretFile != "" || os.Getenv(m.base.SecretEnv) != ""}
	}
	configured := r.Secret != ""
	if r.SecretSource == "server" {
		configured = m.base.SecretFile != "" || os.Getenv(m.base.SecretEnv) != ""
	}
	return PublicSettings{r.Controller, r.RequestTimeoutSeconds, r.SecretSource, configured}
}

func (m *Manager) state() State {
	active, saved := m.active, m.saved
	active.Revision, saved.Revision = 0, 0
	return State{m.saved.Revision, m.saved.Override, m.activeView, m.public(m.saved), active != saved}
}

func (m *Manager) State() State {
	m.mu.Lock()
	defer m.mu.Unlock()
	return m.state()
}

func (m *Manager) prepare(u Update) (record, error) {
	if u.Revision != m.saved.Revision {
		return record{}, ErrConflict
	}
	if u.UseServer {
		return record{Revision: u.Revision + 1}, nil
	}
	next := m.saved
	if !next.Override {
		next.SecretSource = "server"
	}
	next.Override = true
	next.Controller = strings.TrimRight(strings.TrimSpace(u.Controller), "/")
	next.RequestTimeoutSeconds = u.RequestTimeoutSeconds
	next.Revision++
	if u.SecretAction != "replace" && u.Secret != "" {
		return record{}, fmt.Errorf("只有更换密钥时才能提交新密钥")
	}
	switch u.SecretAction {
	case "keep":
	case "replace":
		next.SecretSource, next.Secret = "custom", u.Secret
	case "none":
		next.SecretSource, next.Secret = "none", ""
	case "server":
		next.SecretSource, next.Secret = "server", ""
	default:
		return record{}, fmt.Errorf("请选择有效的密钥操作")
	}
	return next, validate(next)
}

func validate(r record) error {
	u, err := url.Parse(r.Controller)
	if err != nil || len(r.Controller) > 2048 || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(r.Controller, "?#") || strings.IndexFunc(r.Controller, unicode.IsSpace) >= 0 {
		return fmt.Errorf("Controller 地址必须是无用户名、密码、查询参数或片段的完整 HTTP(S) URL")
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return fmt.Errorf("Controller 端口必须在 1–65535 之间")
		}
	}
	if r.RequestTimeoutSeconds < 1 || r.RequestTimeoutSeconds > 30 {
		return fmt.Errorf("连接超时必须在 1–30 秒之间")
	}
	switch r.SecretSource {
	case "server", "none":
		if r.Secret != "" {
			return fmt.Errorf("连接配置文件无效")
		}
	case "custom":
		if strings.TrimSpace(r.Secret) == "" || len(r.Secret) > 4096 || strings.IndexFunc(r.Secret, unicode.IsControl) >= 0 {
			return fmt.Errorf("密钥必须是非空单行文本，且不超过 4096 字节")
		}
	default:
		return fmt.Errorf("请选择有效的密钥操作")
	}
	return nil
}

func (m *Manager) resolve(r record) (config.MihomoConfig, string, error) {
	cfg := m.base
	if r.Override {
		cfg.Controller, cfg.RequestTimeoutSeconds = r.Controller, r.RequestTimeoutSeconds
	}
	if !r.Override || r.SecretSource == "server" {
		secret, err := mihomo.ResolveSecret(m.base)
		if err != nil {
			return cfg, "", fmt.Errorf("无法读取服务器密钥，请检查密钥文件或输入新密钥")
		}
		return cfg, secret, nil
	}
	return cfg, r.Secret, nil
}

func (m *Manager) Test(ctx context.Context, u Update) (string, error) {
	m.mu.Lock()
	next, err := m.prepare(u)
	m.mu.Unlock()
	if err != nil {
		return "", err
	}
	cfg, secret, err := m.resolve(next)
	if err != nil {
		return "", err
	}
	client, err := mihomo.NewWithSecret(cfg, secret)
	if err != nil {
		return "", fmt.Errorf("无法创建 Controller 连接")
	}
	ctx, cancel := context.WithTimeout(ctx, 10*time.Second)
	defer cancel()
	version, err := client.Reachable(ctx)
	if err != nil || strings.TrimSpace(version) == "" || len(version) > 256 {
		return "", fmt.Errorf("连接测试失败，请检查 Controller 地址、密钥和网络；测试最多等待 10 秒")
	}
	return version, nil
}

func (m *Manager) Save(u Update) (State, error) {
	m.mu.Lock()
	defer m.mu.Unlock()
	next, err := m.prepare(u)
	if err != nil {
		return State{}, err
	}
	// Validate local secret availability without requiring an online Controller.
	if _, _, err := m.resolve(next); err != nil {
		return State{}, err
	}
	data, err := json.Marshal(next)
	if err != nil {
		return State{}, fmt.Errorf("无法保存连接配置")
	}
	if err := writePrivate(m.path, data); err != nil {
		return State{}, fmt.Errorf("无法保存连接配置，请检查服务端存储目录权限")
	}
	m.saved = next
	return m.state(), nil
}

// Write one complete record, including the secret, so failures cannot leave a
// new address paired with the old credential. Rename replaces only this file.
func writePrivate(path string, data []byte) error {
	if err := os.MkdirAll(filepath.Dir(path), 0700); err != nil {
		return err
	}
	f, err := os.CreateTemp(filepath.Dir(path), ".connection-*")
	if err != nil {
		return err
	}
	defer os.Remove(f.Name())
	if _, err = f.Write(data); err == nil {
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
