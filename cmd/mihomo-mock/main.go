// Command mihomo-mock is a development-only Mihomo Controller fixture.
// It uses isolated fixtures on workstations or test devices, never live nodes.
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"log"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"
)

type proxy struct {
	Name         string   `json:"name"`
	Type         string   `json:"type"`
	Now          string   `json:"now,omitempty"`
	All          []string `json:"all,omitempty"`
	ProviderName string   `json:"provider-name,omitempty"`
}

type controller struct {
	mu         sync.RWMutex
	proxies    map[string]proxy
	delays     map[string]int
	delayPause time.Duration
}

func main() {
	listen := flag.String("listen", "127.0.0.1:9090", "mock controller listen address")
	delayMS := flag.Int("delay-ms", 0, "delay every healthcheck response for visual QA")
	extraGroups := flag.Int("extra-groups", 0, "additional isolated Selector groups (0-20)")
	extraNodes := flag.Int("extra-nodes", 0, "additional isolated nodes (0-25)")
	flag.Parse()
	if *extraGroups < 0 || *extraGroups > 20 || *extraNodes < 0 || *extraNodes > 25 {
		log.Fatal("invalid fixture size")
	}
	instance := &controller{
		proxies: map[string]proxy{
			"🤖 ChatGPT":   {Name: "🤖 ChatGPT", Type: "Selector", Now: "JP-Tokyo-03", All: []string{"JP-Tokyo-01", "JP-Tokyo-03", "JP-Osaka-02", "US-LA-01", "KR-Seoul-01"}},
			"JP-Tokyo-01": {Name: "JP-Tokyo-01", Type: "Shadowsocks", ProviderName: "Sakura Network"},
			"JP-Tokyo-03": {Name: "JP-Tokyo-03", Type: "VLESS", ProviderName: "Sakura Network"},
			"JP-Osaka-02": {Name: "JP-Osaka-02", Type: "Hysteria2", ProviderName: "Sakura Network"},
			"US-LA-01":    {Name: "US-LA-01", Type: "VLESS", ProviderName: "Pacific Link"},
			"KR-Seoul-01": {Name: "KR-Seoul-01", Type: "Shadowsocks", ProviderName: "Pacific Link"},
		},
		delays:     map[string]int{"JP-Tokyo-01": 151, "JP-Tokyo-03": 109, "JP-Osaka-02": 188, "US-LA-01": 164, "KR-Seoul-01": 132},
		delayPause: time.Duration(*delayMS) * time.Millisecond,
	}
	base := instance.proxies["🤖 ChatGPT"]
	for i := 0; i < *extraNodes; i++ {
		name := fmt.Sprintf("LAB-Node-%02d", i+1)
		instance.proxies[name] = proxy{Name: name, Type: "VLESS"}
		instance.delays[name] = 100 + i
		base.All = append(base.All, name)
	}
	instance.proxies[base.Name] = base
	for i := 0; i < *extraGroups; i++ {
		name := fmt.Sprintf("Mock Group %02d", i+1)
		instance.proxies[name] = proxy{Name: name, Type: "Selector", Now: base.Now, All: append([]string{}, base.All...)}
	}
	mux := http.NewServeMux()
	mux.HandleFunc("/version", instance.version)
	mux.HandleFunc("/proxies", instance.listProxies)
	mux.HandleFunc("/proxies/", instance.proxyRoute)
	mux.HandleFunc("/providers/proxies", instance.listProviders)
	mux.HandleFunc("/providers/proxies/", instance.providerRoute)
	log.Printf("development Mihomo mock listening on http://%s", *listen)
	log.Fatal(http.ListenAndServe(*listen, mux))
}

func (c *controller) version(writer http.ResponseWriter, _ *http.Request) {
	writeJSON(writer, http.StatusOK, map[string]string{"version": "dev-mock"})
}

func (c *controller) listProxies(writer http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	writeJSON(writer, http.StatusOK, map[string]any{"proxies": c.proxies})
}

func (c *controller) listProviders(writer http.ResponseWriter, _ *http.Request) {
	c.mu.RLock()
	defer c.mu.RUnlock()
	first := []proxy{c.proxies["JP-Tokyo-01"], c.proxies["JP-Tokyo-03"], c.proxies["JP-Osaka-02"]}
	second := []proxy{c.proxies["US-LA-01"], c.proxies["KR-Seoul-01"]}
	writeJSON(writer, http.StatusOK, map[string]any{"providers": map[string]any{
		"Sakura Network": map[string]any{"proxies": first},
		"Pacific Link":   map[string]any{"proxies": second},
	}})
}

func (c *controller) proxyRoute(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.TrimPrefix(request.URL.EscapedPath(), "/proxies/"), "/")
	if len(parts) == 2 && parts[1] == "delay" && request.Method == http.MethodGet {
		c.delay(writer, request, unescape(parts[0]))
		return
	}
	if len(parts) == 1 && request.Method == http.MethodPut {
		c.selectProxy(writer, request, unescape(parts[0]))
		return
	}
	http.NotFound(writer, request)
}

func (c *controller) providerRoute(writer http.ResponseWriter, request *http.Request) {
	parts := strings.Split(strings.TrimPrefix(request.URL.EscapedPath(), "/providers/proxies/"), "/")
	if len(parts) == 3 && parts[2] == "healthcheck" && request.Method == http.MethodGet {
		c.delay(writer, request, unescape(parts[1]))
		return
	}
	http.NotFound(writer, request)
}

func (c *controller) delay(writer http.ResponseWriter, _ *http.Request, name string) {
	if c.delayPause > 0 {
		time.Sleep(c.delayPause)
	}
	c.mu.RLock()
	delay, found := c.delays[name]
	c.mu.RUnlock()
	if !found {
		http.Error(writer, "unknown proxy", http.StatusNotFound)
		return
	}
	writeJSON(writer, http.StatusOK, map[string]int{"delay": delay})
}

func (c *controller) selectProxy(writer http.ResponseWriter, request *http.Request, name string) {
	var payload struct {
		Name string `json:"name"`
	}
	if err := json.NewDecoder(request.Body).Decode(&payload); err != nil {
		http.Error(writer, "invalid JSON", http.StatusBadRequest)
		return
	}
	c.mu.Lock()
	defer c.mu.Unlock()
	group, found := c.proxies[name]
	if !found || group.Type != "Selector" {
		http.Error(writer, "unknown selector", http.StatusNotFound)
		return
	}
	for _, member := range group.All {
		if member == payload.Name {
			group.Now = payload.Name
			c.proxies[name] = group
			writer.WriteHeader(http.StatusNoContent)
			return
		}
	}
	http.Error(writer, "node is not a group member", http.StatusBadRequest)
}

func unescape(value string) string {
	decoded, err := url.PathUnescape(value)
	if err != nil {
		return value
	}
	return decoded
}

func writeJSON(writer http.ResponseWriter, status int, payload any) {
	writer.Header().Set("Content-Type", "application/json")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(payload)
}
