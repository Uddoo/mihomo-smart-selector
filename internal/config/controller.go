package config

import (
	"fmt"
	"net/url"
	"strconv"
	"strings"
	"unicode"
)

// ParseControllerURL is shared by YAML configuration, connection drafts and the
// HTTP client. Reject URL components that can hide credentials or change targets.
func ParseControllerURL(value string) (*url.URL, error) {
	u, err := url.Parse(value)
	if err != nil || len(value) > 2048 || u.Hostname() == "" || (u.Scheme != "http" && u.Scheme != "https") || u.User != nil || u.RawQuery != "" || u.ForceQuery || u.Fragment != "" || strings.ContainsAny(value, "?#\\") || strings.IndexFunc(value, unicode.IsSpace) >= 0 {
		return nil, fmt.Errorf("Controller 地址必须是无用户名、密码、查询参数或片段的完整 HTTP(S) URL")
	}
	if port := u.Port(); port != "" {
		value, err := strconv.Atoi(port)
		if err != nil || value < 1 || value > 65535 {
			return nil, fmt.Errorf("Controller 端口必须在 1–65535 之间")
		}
	}
	return u, nil
}

// SameController includes scheme, port and base path: sharing a hostname does
// not authorize sending an existing credential to a different service.
func SameController(a, b string) bool {
	left, err := ParseControllerURL(a)
	if err != nil {
		return false
	}
	right, err := ParseControllerURL(b)
	if err != nil {
		return false
	}
	port := func(u *url.URL) string {
		if u.Port() != "" {
			return u.Port()
		}
		if u.Scheme == "https" {
			return "443"
		}
		return "80"
	}
	return left.Scheme == right.Scheme && strings.EqualFold(left.Hostname(), right.Hostname()) && port(left) == port(right) && strings.TrimRight(left.EscapedPath(), "/") == strings.TrimRight(right.EscapedPath(), "/")
}
