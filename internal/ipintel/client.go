// Package ipintel queries IP intelligence APIs while preserving each source's semantics.
package ipintel

import (
	"context"
	"crypto/hmac"
	"crypto/rand"
	"crypto/sha256"
	"encoding/binary"
	"errors"
	"io"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
)

const CacheTTL = 24 * time.Hour

type cacheKey struct {
	ip         string
	credential [32]byte
}

type Client struct {
	provider     string
	source       string
	scoreKind    string
	http         *http.Client
	endpoint     string
	cacheSecret  [32]byte
	mu           sync.Mutex
	cache        map[cacheKey]model.IPIntelligence
	slot         chan struct{}
	blockedUntil map[[32]byte]time.Time
	now          func() time.Time
}

func NewClient() *Client {
	return newProvider("ipqualityscore")
}

func newProvider(provider string) *Client {
	source, scoreKind, endpoint := "IPQualityScore", "fraud_risk", "https://www.ipqualityscore.com/api/json/ip"
	switch provider {
	case "scamalytics":
		source, scoreKind, endpoint = "Scamalytics", "fraud_risk", "https://api12.scamalytics.com/v3/"
	case "ipapi_is":
		source, scoreKind, endpoint = "ipapi.is", "network_abuse", "https://api.ipapi.is/"
	case "abuseipdb":
		source, scoreKind, endpoint = "AbuseIPDB", "abuse_confidence", "https://api.abuseipdb.com/api/v2/check"
	}
	var cacheSecret [32]byte
	rand.Read(cacheSecret[:])
	return &Client{
		provider: provider, source: source, scoreKind: scoreKind, cacheSecret: cacheSecret,
		http:     &http.Client{Timeout: 6 * time.Second, Transport: &http.Transport{Proxy: http.ProxyFromEnvironment, MaxIdleConns: 2, IdleConnTimeout: time.Minute}, CheckRedirect: func(*http.Request, []*http.Request) error { return http.ErrUseLastResponse }},
		endpoint: endpoint, cache: map[cacheKey]model.IPIntelligence{}, blockedUntil: map[[32]byte]time.Time{}, slot: make(chan struct{}, 1), now: time.Now,
	}
}

func PublicIP(value string) (string, bool) {
	ip, err := netip.ParseAddr(strings.TrimSpace(value))
	if err != nil || ip.Zone() != "" {
		return "", false
	}
	ip = ip.Unmap()
	if !ip.IsGlobalUnicast() || ip.IsPrivate() || ip.IsLoopback() || ip.IsLinkLocalUnicast() {
		return "", false
	}
	return ip.String(), true
}

// Lookup caches by canonical IP and credential, and reserves budget only for
// a real upstream request. Missing credentials never reuse an older score.
func (c *Client) Lookup(ctx context.Context, ip, key string, reserve func() bool) model.IPIntelligence {
	return c.lookup(ctx, ip, credentials{key: key}, reserve)
}

type credentials struct{ key, username, server string }

// credentialID partitions the in-memory cache and backoff, not password storage.
// A random client key prevents reusable unkeyed credential digests. Length prefixes
// distinguish account fields even when a value contains a separator byte.
func (c *Client) credentialID(account credentials) [32]byte {
	mac := hmac.New(sha256.New, c.cacheSecret[:])
	for _, value := range []string{account.key, account.username, account.server} {
		var size [8]byte
		binary.BigEndian.PutUint64(size[:], uint64(len(value)))
		mac.Write(size[:])
		mac.Write([]byte(value))
	}
	var id [32]byte
	copy(id[:], mac.Sum(nil))
	return id
}

func (c *Client) empty(status string) model.IPIntelligence {
	return model.IPIntelligence{Status: status, Provider: c.provider, Source: c.source, ScoreKind: c.scoreKind}
}

func (c *Client) lookup(ctx context.Context, ip string, account credentials, reserve func() bool) model.IPIntelligence {
	result := c.empty("not_configured")
	if account.key == "" {
		return result
	}
	if c.provider == "scamalytics" && !validScamalyticsAccount(account) {
		result.Error = "Scamalytics 需要配置有效用户名与 API 节点"
		return result
	}
	canonical, valid := PublicIP(ip)
	if !valid {
		result.Status, result.Error = "error", "出口 IP 无效"
		return result
	}
	k := cacheKey{canonical, c.credentialID(account)}
	select {
	case c.slot <- struct{}{}:
		defer func() { <-c.slot }()
	case <-ctx.Done():
		result.Status = "cancelled"
		return result
	}
	if ctx.Err() != nil {
		result.Status = "cancelled"
		return result
	}
	now := c.now().UTC()
	c.mu.Lock()
	cached, hit := c.cache[k]
	blocked := now.Before(c.blockedUntil[k.credential])
	c.mu.Unlock()
	if hit && now.Before(cached.ExpiresAt) {
		return cached.Clone()
	}
	if blocked {
		result.Status, result.Error = "cooldown", "IP 情报接口暂缓重试，请核对密钥与配额"
		return result
	}
	if !reserve() {
		result.Status, result.Error = "budget", "后台请求预算不足，IP 情报稍后重试"
		return result
	}
	req, err := c.request(ctx, canonical, account)
	if err != nil {
		result.Status, result.Error = "error", "IP 情报请求失败"
		return result
	}
	// Never forward raw transport errors: Scamalytics authenticates in its URL.
	resp, err := c.http.Do(req)
	result.Status = "error"
	if err != nil {
		result.Error = "IP 情报请求失败"
		return result
	}
	defer resp.Body.Close()
	if resp.StatusCode == http.StatusTooManyRequests {
		c.mu.Lock()
		c.blockedUntil[k.credential] = retryAfter(resp.Header.Get("Retry-After"), now)
		c.mu.Unlock()
		result.Status, result.Error = "rate_limited", "IP 情报接口限额已用尽，稍后重试"
		return result
	}
	if resp.StatusCode != http.StatusOK {
		if resp.StatusCode == http.StatusUnauthorized || resp.StatusCode == http.StatusForbidden {
			c.mu.Lock()
			c.blockedUntil[k.credential] = now.Add(time.Hour)
			c.mu.Unlock()
		}
		result.Error = "IP 情报接口拒绝请求，请核对密钥与配额"
		return result
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, (256<<10)+1))
	if err != nil || len(body) > 256<<10 {
		result.Error = "IP 情报响应无效"
		return result
	}
	data, decodeErr := c.decode(body, canonical)
	if decodeErr != nil {
		result.Error = "IP 情报响应无效"
		if errors.Is(decodeErr, errAccount) || errors.Is(decodeErr, errQuota) {
			c.mu.Lock()
			c.blockedUntil[k.credential] = retryAfter(resp.Header.Get("Retry-After"), now)
			c.mu.Unlock()
			result.Error = "IP 情报查询失败，请核对密钥与配额"
			if errors.Is(decodeErr, errQuota) {
				result.Status, result.Error = "rate_limited", "IP 情报接口限额已用尽，稍后重试"
			}
		}
		return result
	}
	result = data
	result.Provider, result.Source, result.ScoreKind, result.Status = c.provider, c.source, c.scoreKind, "ok"
	result.FetchedAt = c.now().UTC()
	result.ExpiresAt = result.FetchedAt.Add(CacheTTL)
	c.mu.Lock()
	if len(c.cache) >= 1024 {
		clear(c.cache)
	}
	c.cache[k] = result.Clone()
	c.mu.Unlock()
	return result
}

func retryAfter(value string, now time.Time) time.Time {
	until := now.Add(time.Hour)
	if seconds, err := strconv.ParseInt(value, 10, 32); err == nil && seconds > 0 {
		if candidate := now.Add(time.Duration(seconds) * time.Second); candidate.After(until) {
			until = candidate
		}
	} else if candidate, err := http.ParseTime(value); err == nil && candidate.After(until) {
		until = candidate
	}
	return until
}

func clean(value string) string {
	value = strings.TrimSpace(value)
	if value == "N/A" || value == "Unknown" || strings.HasPrefix(value, "PREMIUM FIELD") {
		return ""
	}
	if len(value) > 256 {
		return ""
	}
	return value
}
