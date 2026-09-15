package api

import (
	"encoding/json"
	"fmt"
	"net/url"
	"regexp"
	"slices"
	"strings"
)

// The catalog is compiled with the frontend. It is not a user-controlled proxy
// destination list: the server never sends these requests or attaches secrets.
func connectivitySources(data []byte) (string, error) {
	var targets []struct {
		URL string `json:"url"`
	}
	if err := json.Unmarshal(data, &targets); err != nil {
		return "", err
	}
	hostPattern := regexp.MustCompile(`^[a-z0-9]+(?:[.-][a-z0-9]+)*\.[a-z]{2,}$`)
	origins := make(map[string]bool)
	for _, target := range targets {
		u, err := url.Parse(target.URL)
		if err != nil || u.Scheme != "https" || u.User != nil || u.Port() != "" || !hostPattern.MatchString(u.Host) {
			return "", fmt.Errorf("invalid connectivity target")
		}
		origins["https://"+u.Host] = true
	}
	ordered := make([]string, 0, len(origins))
	for origin := range origins {
		ordered = append(ordered, origin)
	}
	slices.Sort(ordered)
	return strings.Join(ordered, " "), nil
}

func embeddedConnectivitySources() string {
	data, err := embeddedStatic.ReadFile("static/connectivity-targets.json")
	if err != nil {
		return ""
	}
	sources, err := connectivitySources(data)
	if err != nil {
		return ""
	} // Fail closed if a build contains an invalid catalog.
	return sources
}

var browserConnectivitySources = embeddedConnectivitySources()
