package api

import (
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"net/http"
)

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
