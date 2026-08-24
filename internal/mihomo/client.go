package mihomo

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"strings"
	"time"

	"github.com/yw-li/mihomo-smart-selector/internal/config"
)

type Proxy struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Now          string   `json:"now,omitempty"`
	All          []string `json:"all,omitempty"`
	ProviderName string   `json:"provider-name,omitempty"`
	Alive        bool     `json:"alive"`
}

type Provider struct {
	Name    string  `json:"name"`
	Proxies []Proxy `json:"proxies"`
}

type Client interface {
	ListProxies(context.Context) (map[string]Proxy, error)
	ListProviders(context.Context) ([]Provider, error)
	Delay(context.Context, string, string, config.Probe, int) (int, error)
	Select(context.Context, string, string) error
	Reachable(context.Context) (string, error)
}

type HTTPClient struct {
	baseURL *url.URL
	secret  string
	client  *http.Client
}

func New(cfg config.MihomoConfig) (*HTTPClient, error) {
	baseURL, err := url.Parse(cfg.Controller)
	if err != nil {
		return nil, fmt.Errorf("parse controller URL: %w", err)
	}
	secret := os.Getenv(cfg.SecretEnv)
	return &HTTPClient{
		baseURL: baseURL,
		secret:  secret,
		client:  &http.Client{Timeout: time.Duration(cfg.RequestTimeoutSeconds) * time.Second},
	}, nil
}

func (c *HTTPClient) Reachable(ctx context.Context) (string, error) {
	response, err := c.request(ctx, http.MethodGet, []string{"version"}, nil)
	if err != nil {
		return "", err
	}
	defer response.Body.Close()
	var payload struct {
		Version string `json:"version"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return "", fmt.Errorf("decode Mihomo version: %w", err)
	}
	return payload.Version, nil
}

func (c *HTTPClient) ListProxies(ctx context.Context) (map[string]Proxy, error) {
	response, err := c.request(ctx, http.MethodGet, []string{"proxies"}, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var payload struct {
		Proxies map[string]Proxy `json:"proxies"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode proxies: %w", err)
	}
	for name, proxy := range payload.Proxies {
		if proxy.Name == "" {
			proxy.Name = name
			payload.Proxies[name] = proxy
		}
	}
	return payload.Proxies, nil
}

func (c *HTTPClient) ListProviders(ctx context.Context) ([]Provider, error) {
	response, err := c.request(ctx, http.MethodGet, []string{"providers", "proxies"}, nil)
	if err != nil {
		return nil, err
	}
	defer response.Body.Close()
	var payload struct {
		Providers map[string]struct {
			Proxies []Proxy `json:"proxies"`
		} `json:"providers"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 16<<20)).Decode(&payload); err != nil {
		return nil, fmt.Errorf("decode proxy providers: %w", err)
	}
	providers := make([]Provider, 0, len(payload.Providers))
	for name, provider := range payload.Providers {
		providers = append(providers, Provider{Name: name, Proxies: provider.Proxies})
	}
	return providers, nil
}

func (c *HTTPClient) Delay(ctx context.Context, name, provider string, probe config.Probe, timeoutMS int) (int, error) {
	segments := []string{"proxies", name, "delay"}
	queries := url.Values{
		"url":     []string{probe.URL},
		"timeout": []string{fmt.Sprintf("%d", timeoutMS)},
	}
	if provider == "" {
		queries.Set("expected", probe.ExpectedStatus)
	} else {
		// Provider-owned nodes may be intentionally omitted from /proxies by
		// Smart controller builds. Their documented health-check endpoint does
		// not accept expected-status, so success here means reachability via the
		// configured probe URL rather than a verified response-code match.
		segments = []string{"providers", "proxies", provider, name, "healthcheck"}
	}
	response, err := c.request(ctx, http.MethodGet, segments, nil, queries)
	if err != nil {
		return 0, err
	}
	defer response.Body.Close()
	var payload struct {
		Delay int `json:"delay"`
	}
	if err := json.NewDecoder(io.LimitReader(response.Body, 1<<20)).Decode(&payload); err != nil {
		return 0, fmt.Errorf("decode delay for proxy %q: %w", name, err)
	}
	if payload.Delay <= 0 {
		return 0, fmt.Errorf("controller returned non-positive delay for proxy %q", name)
	}
	return payload.Delay, nil
}

func (c *HTTPClient) Select(ctx context.Context, group, member string) error {
	body, err := json.Marshal(struct {
		Name string `json:"name"`
	}{Name: member})
	if err != nil {
		return fmt.Errorf("encode selector request: %w", err)
	}
	response, err := c.request(ctx, http.MethodPut, []string{"proxies", group}, bytes.NewReader(body))
	if err != nil {
		return err
	}
	response.Body.Close()
	return nil
}

func (c *HTTPClient) request(ctx context.Context, method string, segments []string, body io.Reader, queries ...url.Values) (*http.Response, error) {
	target := *c.baseURL
	basePath := strings.TrimSuffix(target.Path, "/")
	rawBasePath := strings.TrimSuffix(target.EscapedPath(), "/")
	for _, segment := range segments {
		basePath += "/" + segment
		rawBasePath += "/" + url.PathEscape(segment)
	}
	target.Path = basePath
	target.RawPath = rawBasePath
	if len(queries) > 0 {
		target.RawQuery = queries[0].Encode()
	}
	request, err := http.NewRequestWithContext(ctx, method, target.String(), body)
	if err != nil {
		return nil, fmt.Errorf("build controller request: %w", err)
	}
	request.Header.Set("Accept", "application/json")
	if body != nil {
		request.Header.Set("Content-Type", "application/json")
	}
	if c.secret != "" {
		request.Header.Set("Authorization", "Bearer "+c.secret)
	}
	response, err := c.client.Do(request)
	if err != nil {
		return nil, fmt.Errorf("controller request failed: %w", err)
	}
	if response.StatusCode < 200 || response.StatusCode >= 300 {
		response.Body.Close()
		return nil, fmt.Errorf("controller returned HTTP %d for %s", response.StatusCode, strings.Join(segments, "/"))
	}
	return response, nil
}
