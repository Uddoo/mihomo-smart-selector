package api

import (
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"net/http"
	"strconv"
	"strings"
)

func (s *Server) route(writer http.ResponseWriter, request *http.Request) {
	if s.service != nil && s.service.pending.Load() && strings.HasPrefix(request.URL.Path, "/api/") && request.URL.Path != "/api/v1/service" && request.URL.Path != "/api/v1/service/restart" {
		writer.Header().Set("Retry-After", "1")
		writeError(writer, http.StatusServiceUnavailable, "服务正在重启，请等待恢复")
		return
	}
	switch {
	case request.URL.Path == "/api/v1/service" || request.URL.Path == "/api/v1/service/restart":
		s.serviceRoute(writer, request)
	case request.URL.Path == "/api/v1/connection" || request.URL.Path == "/api/v1/connection/test":
		s.connectionRoute(writer, request)
	case request.URL.Path == "/api/v1/monitor" || strings.HasPrefix(request.URL.Path, "/api/v1/monitor/"):
		s.monitorRoute(writer, request)
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
