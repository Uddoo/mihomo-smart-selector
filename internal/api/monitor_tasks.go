package api

import (
	"errors"
	"net/http"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
)

func (s *Server) monitorTaskRoute(w http.ResponseWriter, r *http.Request) {
	const root = "/api/v1/monitor/tasks"
	if r.URL.Path == root {
		switch r.Method {
		case http.MethodGet:
			tasks, err := s.monitor.Tasks(r.Context())
			if err != nil {
				writeError(w, 500, "无法读取监控任务")
				return
			}
			if r.URL.Query().Get("include") == "scheduler" {
				writeJSON(w, 200, struct {
					Tasks     []model.MonitorTask     `json:"tasks"`
					Scheduler monitor.SchedulerStatus `json:"scheduler"`
				}{tasks, s.monitor.SchedulerStatus()})
			} else {
				writeJSON(w, 200, tasks)
			}
		case http.MethodPost:
			s.saveMonitorTask(w, r, "", 201)
		default:
			writeError(w, 405, "监控任务接口不支持此方法")
		}
		return
	}
	parts := strings.Split(strings.TrimPrefix(r.URL.Path, root+"/"), "/")
	if parts[0] == "" {
		writeError(w, 404, "监控任务接口不存在")
		return
	}
	taskID := parts[0]
	if len(parts) > 1 && parts[1] != "revisions" {
		runtime, err := s.monitor.ForTask(r.Context(), taskID)
		if err != nil {
			writeMonitorTaskError(w, err)
			return
		}
		path := strings.Join(parts[1:], "/")
		switch path {
		case "overview":
			path = ""
		case "failover", "retest", "series", "incidents", "correlations", "diagnostics":
		default:
			if len(parts) != 4 || parts[1] != "nodes" || parts[2] == "" || parts[3] != "timeline" {
				writeError(w, 404, "监控任务接口不存在")
				return
			}
		}
		request := r.Clone(r.Context())
		request.URL.Path = "/api/v1/monitor"
		if path != "" {
			request.URL.Path += "/" + path
		}
		s.serveMonitor(w, request, runtime, true)
		return
	}
	if len(parts) > 2 {
		writeError(w, 404, "监控任务接口不存在")
		return
	}
	if len(parts) == 2 {
		if r.Method != http.MethodGet {
			writeError(w, 405, "监控任务接口不支持此方法")
			return
		}
		revisions, err := s.monitor.TaskRevisions(r.Context(), taskID)
		if err != nil {
			writeMonitorTaskError(w, err)
			return
		}
		writeJSON(w, 200, revisions)
		return
	}
	switch r.Method {
	case http.MethodGet:
		task, err := s.monitor.Task(r.Context(), taskID)
		if err != nil {
			writeMonitorTaskError(w, err)
			return
		}
		writeJSON(w, 200, task)
	case http.MethodPut:
		s.saveMonitorTask(w, r, taskID, 200)
	default:
		writeError(w, 405, "监控任务接口不支持此方法")
	}
}

func (s *Server) saveMonitorTask(w http.ResponseWriter, r *http.Request, taskID string, status int) {
	var body struct {
		model.MonitorRequest
		Enabled *bool `json:"enabled"`
	}
	if !decodeJSON(w, r, &body) {
		return
	}
	if body.Enabled == nil {
		writeError(w, 400, "enabled is required")
		return
	}
	body.MonitorRequest.Enabled = *body.Enabled
	if taskID == "" && body.Revision != 0 {
		writeError(w, 409, history.ErrMonitorRevision.Error())
		return
	}
	plan, err := s.monitor.SaveTask(r.Context(), taskID, body.MonitorRequest)
	if err != nil {
		writeMonitorTaskError(w, err)
		return
	}
	writeJSON(w, status, plan)
}

func writeMonitorTaskError(w http.ResponseWriter, err error) {
	if errors.Is(err, history.ErrMonitorTaskNotFound) {
		writeError(w, 404, err.Error())
		return
	}
	writeError(w, 409, err.Error())
}
