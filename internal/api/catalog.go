package api

import (
	"fmt"
	"net/http"
)

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
