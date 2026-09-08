package api

import (
	"crypto/subtle"
	"embed"
	"encoding/json"
	"fmt"
	"io"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
	"strconv"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
)

//go:embed static/*
var embeddedStatic embed.FS

type Server struct {
	config     config.HTTPConfig
	manager    *scan.Manager
	controller mihomo.Client
	static     http.Handler
	allowed    []netip.Prefix
	exposed    bool
}

func New(cfg config.HTTPConfig, manager *scan.Manager, controller mihomo.Client) (*Server, error) {
	staticFiles, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	host, _, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("parse HTTP listener: %w", err)
	}
	exposed := host != "localhost"
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		exposed = !ip.IsLoopback()
	}
	server := &Server{
		config: cfg, manager: manager, controller: controller,
		static: http.FileServer(http.FS(staticFiles)), exposed: exposed,
	}
	for _, cidr := range cfg.AllowedCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse allowed CIDR: %w", err)
		}
		server.allowed = append(server.allowed, prefix)
	}
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.authorize(http.HandlerFunc(s.route)))
}

func (s *Server) route(writer http.ResponseWriter, request *http.Request) {
	switch {
	case request.URL.Path == "/api/v1/scans" && request.Method == http.MethodGet:
		items, err := s.manager.Recent(request.Context())
		if err != nil {
			writeError(writer, 500, "could not read recent scans")
			return
		}
		writeJSON(writer, 200, items)
	case request.URL.Path == "/api/v1/storage" && request.Method == http.MethodGet:
		stats, err := s.manager.Storage(request.Context())
		if err != nil {
			writeError(writer, 500, "could not read storage stats")
			return
		}
		writeJSON(writer, 200, stats)
	case request.URL.Path == "/api/v1/storage/cleanup" && request.Method == http.MethodPost:
		var payload struct {
			Confirm  bool `json:"confirm"`
			Revision int  `json:"revision"`
		}
		if !decodeJSON(writer, request, &payload) {
			return
		}
		if !payload.Confirm {
			writeError(writer, 400, "cleanup confirmation required")
			return
		}
		result, err := s.manager.Cleanup(request.Context(), payload.Revision)
		if err != nil {
			writeError(writer, 409, err.Error())
			return
		}
		writeJSON(writer, 200, result)
	case strings.HasPrefix(request.URL.Path, "/api/v1/history/") && request.Method == http.MethodPost:
		parts := strings.Split(strings.TrimPrefix(request.URL.Path, "/api/v1/history/"), "/")
		if len(parts) != 2 || parts[1] != "reconcile" {
			writeError(writer, 404, "history route not found")
			return
		}
		id, err := strconv.ParseInt(parts[0], 10, 64)
		if err != nil {
			writeError(writer, 400, "invalid operation id")
			return
		}
		result, err := s.manager.ReconcileSwitch(request.Context(), id)
		if err != nil {
			writeError(writer, 409, err.Error())
			return
		}
		writeJSON(writer, 200, result)
	case request.URL.Path == "/api/v1/settings" && request.Method == http.MethodGet:
		writeJSON(writer, http.StatusOK, s.manager.Settings())
	case request.URL.Path == "/api/v1/settings" && request.Method == http.MethodPut:
		var payload config.RuntimeSettings
		if !decodeJSON(writer, request, &payload) {
			return
		}
		settings, err := s.manager.SaveSettings(request.Context(), payload)
		if err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, settings)
	case request.URL.Path == "/api/v1/health" && request.Method == http.MethodGet:
		s.health(writer, request)
	case request.URL.Path == "/api/v1/groups" && request.Method == http.MethodGet:
		s.groups(writer, request)
	case request.URL.Path == "/api/v1/services" && request.Method == http.MethodGet:
		catalog, err := s.manager.Services(request.Context())
		if err != nil {
			writeError(writer, http.StatusBadGateway, "could not load service catalog and bindings")
			return
		}
		writeJSON(writer, http.StatusOK, catalog)
	case request.URL.Path == "/api/v1/bindings" && request.Method == http.MethodPut:
		var item model.ServiceBinding
		if !decodeJSON(writer, request, &item) {
			return
		}
		if err := s.manager.SetBinding(request.Context(), item); err != nil {
			writeError(writer, http.StatusBadRequest, err.Error())
			return
		}
		writeJSON(writer, http.StatusOK, map[string]bool{"saved": true})
	case request.URL.Path == "/api/v1/providers" && request.Method == http.MethodGet:
		s.providers(writer, request)
	case request.URL.Path == "/api/v1/regions" && request.Method == http.MethodGet:
		writeJSON(writer, http.StatusOK, s.manager.Regions())
	case request.URL.Path == "/api/v1/nodes" && request.Method == http.MethodGet:
		s.nodes(writer, request)
	case request.URL.Path == "/api/v1/history" && request.Method == http.MethodGet:
		s.history(writer, request)
	case request.URL.Path == "/api/v1/scans" && request.Method == http.MethodPost:
		s.createScan(writer, request)
	case request.URL.Path == "/api/v1/scans/preflight" && request.Method == http.MethodPost:
		s.preflightScan(writer, request)
	case strings.HasPrefix(request.URL.Path, "/api/v1/scans/"):
		s.scanRoute(writer, request)
	case strings.HasPrefix(request.URL.Path, "/api/"):
		writeError(writer, http.StatusNotFound, "API route not found")
	default:
		s.static.ServeHTTP(writer, request)
	}
}

func (s *Server) health(writer http.ResponseWriter, request *http.Request) {
	version, err := s.controller.Reachable(request.Context())
	if err != nil {
		writeJSON(writer, http.StatusServiceUnavailable, map[string]any{
			"status": "degraded", "mihomo_connected": false, "error": "Mihomo Controller is unreachable",
		})
		return
	}
	writeJSON(writer, http.StatusOK, map[string]any{
		"status": "ok", "mihomo_connected": true, "mihomo_version": version,
	})
}

func (s *Server) groups(writer http.ResponseWriter, request *http.Request) {
	groups, err := s.manager.Groups(request.Context())
	if err != nil {
		writeError(writer, http.StatusBadGateway, "could not list Mihomo selector groups")
		return
	}
	writeJSON(writer, http.StatusOK, groups)
}

func (s *Server) providers(writer http.ResponseWriter, request *http.Request) {
	providers, err := s.manager.Providers(request.Context())
	if err != nil {
		writeError(writer, http.StatusBadGateway, "could not list Mihomo providers")
		return
	}
	writeJSON(writer, http.StatusOK, providers)
}

func (s *Server) nodes(writer http.ResponseWriter, request *http.Request) {
	nodes, err := s.manager.Nodes(request.Context())
	if err != nil {
		writeError(writer, http.StatusBadGateway, "could not list Mihomo nodes")
		return
	}
	writeJSON(writer, http.StatusOK, nodes)
}

func (s *Server) history(writer http.ResponseWriter, request *http.Request) {
	limit := 20
	if value := request.URL.Query().Get("limit"); value != "" {
		if _, err := fmt.Sscanf(value, "%d", &limit); err != nil {
			writeError(writer, http.StatusBadRequest, "limit must be a number")
			return
		}
	}
	events, err := s.manager.History(request.Context(), limit)
	if err != nil {
		writeError(writer, http.StatusInternalServerError, "could not read history")
		return
	}
	writeJSON(writer, http.StatusOK, events)
}

func (s *Server) createScan(writer http.ResponseWriter, request *http.Request) {
	var payload model.ScanRequest
	if !decodeJSON(writer, request, &payload) {
		return
	}
	scan, err := s.manager.Start(payload)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusAccepted, scan)
}

func (s *Server) preflightScan(writer http.ResponseWriter, request *http.Request) {
	var payload model.ScanRequest
	if !decodeJSON(writer, request, &payload) {
		return
	}
	preview, err := s.manager.Preflight(request.Context(), payload)
	if err != nil {
		writeError(writer, http.StatusBadRequest, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, preview)
}

func (s *Server) scanRoute(writer http.ResponseWriter, request *http.Request) {
	rest := strings.TrimPrefix(request.URL.Path, "/api/v1/scans/")
	parts := strings.Split(rest, "/")
	if len(parts) == 1 && request.Method == http.MethodGet {
		s.getScan(writer, request, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "events" && request.Method == http.MethodGet {
		s.events(writer, request, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "select" && request.Method == http.MethodPost {
		s.selectNode(writer, request, parts[0])
		return
	}
	if len(parts) == 2 && parts[1] == "retest" && request.Method == http.MethodPost {
		var payload struct {
			Node string `json:"node"`
		}
		if !decodeJSON(writer, request, &payload) {
			return
		}
		result, err := s.manager.Retest(request.Context(), parts[0], payload.Node)
		if err != nil {
			writeError(writer, http.StatusConflict, err.Error())
			return
		}
		writeJSON(writer, http.StatusAccepted, result)
		return
	}
	if len(parts) == 2 && parts[1] == "stop" && request.Method == http.MethodPost {
		s.stopScan(writer, request, parts[0])
		return
	}
	writeError(writer, http.StatusNotFound, "scan route not found")
}

func (s *Server) getScan(writer http.ResponseWriter, request *http.Request, id string) {
	scan, err := s.manager.Get(request.Context(), id)
	if err != nil {
		writeError(writer, http.StatusNotFound, "scan not found")
		return
	}
	writeJSON(writer, http.StatusOK, scan)
}

func (s *Server) selectNode(writer http.ResponseWriter, request *http.Request, id string) {
	var payload struct {
		Node      string `json:"node"`
		RequestID string `json:"request_id"`
	}
	if !decodeJSON(writer, request, &payload) {
		return
	}
	var event model.SwitchEvent
	var err error
	if payload.RequestID == "" {
		event, err = s.manager.Select(request.Context(), id, payload.Node)
	} else {
		event, err = s.manager.SelectRequest(request.Context(), id, payload.Node, payload.RequestID)
	}
	if err != nil {
		writeError(writer, http.StatusConflict, err.Error())
		return
	}
	writeJSON(writer, http.StatusOK, event)
}

func (s *Server) stopScan(writer http.ResponseWriter, request *http.Request, id string) {
	var payload struct {
		AfterCurrentBatch bool `json:"after_current_batch"`
	}
	if !decodeJSON(writer, request, &payload) {
		return
	}
	progress, err := s.manager.Stop(id, payload.AfterCurrentBatch)
	if err != nil {
		writeError(writer, http.StatusConflict, err.Error())
		return
	}
	writeJSON(writer, http.StatusAccepted, map[string]any{"progress": progress})
}

func (s *Server) events(writer http.ResponseWriter, request *http.Request, id string) {
	if _, err := s.manager.Get(request.Context(), id); err != nil {
		writeError(writer, http.StatusNotFound, "scan not found")
		return
	}
	flusher, supported := writer.(http.Flusher)
	if !supported {
		writeError(writer, http.StatusInternalServerError, "streaming is not supported")
		return
	}
	writer.Header().Set("Content-Type", "text/event-stream")
	writer.Header().Set("Cache-Control", "no-cache")
	writer.Header().Set("Connection", "keep-alive")
	events, unsubscribe := s.manager.Subscribe(id)
	defer unsubscribe()
	writeEvent(writer, flusher, scan.Event{Kind: "connected", Message: "event stream connected", At: time.Now().UTC()})
	for {
		select {
		case <-request.Context().Done():
			return
		case event := <-events:
			writeEvent(writer, flusher, event)
		case <-time.After(15 * time.Second):
			_, _ = io.WriteString(writer, ": keepalive\n\n")
			flusher.Flush()
		}
	}
}

func writeEvent(writer http.ResponseWriter, flusher http.Flusher, event scan.Event) {
	payload, err := json.Marshal(event)
	if err != nil {
		return
	}
	_, _ = fmt.Fprintf(writer, "event: %s\ndata: %s\n\n", event.Kind, payload)
	flusher.Flush()
}

func decodeJSON(writer http.ResponseWriter, request *http.Request, output any) bool {
	request.Body = http.MaxBytesReader(writer, request.Body, 64<<10)
	defer request.Body.Close()
	decoder := json.NewDecoder(request.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(output); err != nil {
		writeError(writer, http.StatusBadRequest, "invalid JSON request")
		return false
	}
	if decoder.Decode(&struct{}{}) != io.EOF {
		writeError(writer, http.StatusBadRequest, "request body must contain one JSON value")
		return false
	}
	return true
}

func writeJSON(writer http.ResponseWriter, status int, value any) {
	writer.Header().Set("Content-Type", "application/json; charset=utf-8")
	writer.WriteHeader(status)
	_ = json.NewEncoder(writer).Encode(value)
}

func writeError(writer http.ResponseWriter, status int, message string) {
	writeJSON(writer, status, map[string]string{"error": message})
}

func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// The SPA shell contains no router/controller data and must remain
		// reachable so a LAN user can enter the session-only API token. All
		// application data and state-changing operations remain under /api/.
		if !s.exposed || !strings.HasPrefix(request.URL.Path, "/api/") {
			next.ServeHTTP(writer, request)
			return
		}
		address, err := netip.ParseAddrPort(request.RemoteAddr)
		if err != nil || !s.isAllowed(address.Addr()) {
			writeError(writer, http.StatusForbidden, "source address is not permitted")
			return
		}
		if s.config.AllowUnauthenticatedLAN {
			next.ServeHTTP(writer, request)
			return
		}
		const prefix = "Bearer "
		token := strings.TrimPrefix(request.Header.Get("Authorization"), prefix)
		if token == request.Header.Get("Authorization") ||
			subtle.ConstantTimeCompare([]byte(token), []byte(s.config.APIToken)) != 1 {
			writer.Header().Set("WWW-Authenticate", `Bearer realm="Mihomo Smart Selector"`)
			writeError(writer, http.StatusUnauthorized, "valid API token is required")
			return
		}
		next.ServeHTTP(writer, request)
	})
}

func (s *Server) isAllowed(address netip.Addr) bool {
	// The service itself and local health probes use loopback. It is not a
	// remote-access bypass; all LAN-originated API calls still need both the
	// configured CIDR and Bearer token.
	if address.IsLoopback() {
		return true
	}
	for _, prefix := range s.allowed {
		if prefix.Contains(address) {
			return true
		}
	}
	return false
}

func (s *Server) securityHeaders(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		writer.Header().Set("X-Content-Type-Options", "nosniff")
		writer.Header().Set("X-Frame-Options", "DENY")
		writer.Header().Set("Referrer-Policy", "no-referrer")
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self'; style-src 'self' 'unsafe-inline'; script-src 'self'")
		next.ServeHTTP(writer, request)
	})
}
