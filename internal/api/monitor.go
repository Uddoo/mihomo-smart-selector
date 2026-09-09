package api

import (
	"context"
	"encoding/base64"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
)

func (s *Server) WithMonitor(m *monitor.Manager) *Server { s.monitor = m; return s }

func (s *Server) monitorRoute(w http.ResponseWriter, r *http.Request) {
	if s.monitor == nil {
		writeError(w, 503, "监控服务未启动")
		return
	}
	switch {
	case r.URL.Path == "/api/v1/monitor/storage" && r.Method == http.MethodGet:
		out, err := s.monitor.Storage(r.Context())
		if err != nil {
			writeError(w, 500, "无法读取监控存储状态")
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/retention" && r.Method == http.MethodPut:
		var body model.MonitorRetention
		if !decodeJSON(w, r, &body) {
			return
		}
		out, err := s.monitor.SaveRetention(r.Context(), body)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/correlations" && r.Method == http.MethodGet:
		out, err := s.monitor.Correlations(r.Context())
		if err != nil {
			writeError(w, 500, "无法读取关联事件")
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/incidents" && r.Method == http.MethodGet:
		from, to, err := monitorRange(r)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		limit := 100
		if value := r.URL.Query().Get("limit"); value != "" {
			limit, err = strconv.Atoi(value)
			if err != nil {
				writeError(w, 400, "无效分页大小")
				return
			}
		}
		out, err := s.monitor.Activities(r.Context(), from, to, r.URL.Query().Get("cursor"), limit)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/diagnostics" && r.Method == http.MethodPost:
		var body monitor.DiagnosticRequest
		if !decodeJSON(w, r, &body) {
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 20*time.Second)
		defer cancel()
		data, err := s.monitor.Diagnostics(ctx, body)
		if err != nil {
			if errors.Is(err, monitor.ErrHistoryBusy) {
				writeError(w, 503, err.Error())
				return
			}
			writeError(w, 400, err.Error())
			return
		}
		// Browser fetches use a JSON envelope so download-manager extensions do
		// not intercept an authenticated background request as a file transfer.
		if strings.Contains(r.Header.Get("Accept"), "application/json") {
			w.Header().Set("Cache-Control", "no-store")
			writeJSON(w, 200, map[string]string{"filename": "mihomo-monitor-diagnostics.zip", "content_type": "application/zip", "encoding": "base64", "data": base64.StdEncoding.EncodeToString(data)})
			return
		}
		w.Header().Set("Content-Type", "application/zip")
		w.Header().Set("Content-Disposition", `attachment; filename="mihomo-monitor-diagnostics.zip"`)
		w.Header().Set("Content-Length", strconv.Itoa(len(data)))
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(200)
		_, _ = w.Write(data)
	case r.URL.Path == "/api/v1/monitor/series" && r.Method == http.MethodGet:
		out, err := s.monitor.Series(r.Context())
		if err != nil {
			writeError(w, 500, "无法读取历史序列")
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/revisions" && r.Method == http.MethodGet:
		out, err := s.monitor.Revisions(r.Context())
		if err != nil {
			writeError(w, 500, "无法读取方案修订")
			return
		}
		writeJSON(w, 200, out)
	case strings.HasPrefix(r.URL.Path, "/api/v1/monitor/nodes/") && strings.HasSuffix(r.URL.Path, "/timeline") && r.Method == http.MethodGet:
		id := strings.TrimSuffix(strings.TrimPrefix(r.URL.Path, "/api/v1/monitor/nodes/"), "/timeline")
		from, to, err := monitorRange(r)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		out, err := s.monitor.Timeline(r.Context(), id, from, to)
		if err != nil {
			if errors.Is(err, monitor.ErrHistoryBusy) {
				writeError(w, 503, err.Error())
				return
			}
			writeError(w, 400, err.Error())
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/failover" && r.Method == http.MethodPut:
		var body struct {
			Revision int   `json:"revision"`
			Enabled  *bool `json:"enabled"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if body.Enabled == nil {
			writeError(w, 400, "enabled is required")
			return
		}
		p, err := s.monitor.SetAutoSwitch(r.Context(), body.Revision, *body.Enabled)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, 200, p)
	case r.URL.Path == "/api/v1/monitor" && r.Method == http.MethodGet:
		if _, err := monitor.WindowDuration(r.URL.Query().Get("window")); err != nil {
			writeError(w, 400, err.Error())
			return
		}
		out, err := s.monitor.WindowOverview(r.Context(), r.URL.Query().Get("window"))
		if err != nil {
			if errors.Is(err, monitor.ErrHistoryBusy) {
				writeError(w, 503, err.Error())
				return
			}
			writeError(w, 500, "无法读取监控历史")
			return
		}
		writeJSON(w, 200, out)
	case r.URL.Path == "/api/v1/monitor/plan" && r.Method == http.MethodPut:
		var body model.MonitorRequest
		if !decodeJSON(w, r, &body) {
			return
		}
		p, err := s.monitor.Save(r.Context(), body)
		if err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, 200, p)
	case r.URL.Path == "/api/v1/monitor/retest" && r.Method == http.MethodPost:
		var body struct {
			NodeID   string `json:"node_id"`
			Revision int    `json:"revision"`
		}
		if !decodeJSON(w, r, &body) {
			return
		}
		if err := s.monitor.Retest(body.NodeID, body.Revision); err != nil {
			writeError(w, 409, err.Error())
			return
		}
		writeJSON(w, 202, map[string]bool{"queued": true})
	case r.URL.Path == "/api/v1/monitor/catalog" && r.Method == http.MethodGet:
		group, profile := r.URL.Query().Get("group"), r.URL.Query().Get("profile_id")
		p, nodes, current, err := s.manager.MonitorCatalog(r.Context(), group, profile)
		if err != nil {
			writeError(w, 400, err.Error())
			return
		}
		suggested := []string{}
		seen := map[string]bool{}
		present := map[string]bool{}
		for _, n := range nodes {
			present[n.Name] = true
		}
		add := func(name string) {
			if present[name] && !seen[name] && len(suggested) < 6 {
				suggested = append(suggested, name)
				seen[name] = true
			}
		}
		add(current)
		recent, err := s.manager.Recent(r.Context())
		if err == nil {
			for _, item := range recent {
				if item.Status == model.ScanComplete && item.Request.TargetGroup == group && item.Request.ProfileID == profile {
					full, err := s.manager.Get(r.Context(), item.ID)
					if err == nil {
						for _, n := range full.Results {
							if n.SuccessRate > 0 {
								add(n.Name)
							}
						}
					}
					break
				}
			}
		}
		writeJSON(w, 200, map[string]any{"nodes": nodes, "current": current, "suggested": suggested, "probe_count": len(p.Probes)})
	default:
		writeError(w, 404, "监控接口不存在")
	}
}

func monitorRange(r *http.Request) (time.Time, time.Time, error) {
	to := time.Now().UTC()
	var err error
	if value := r.URL.Query().Get("to"); value != "" {
		to, err = time.Parse(time.RFC3339, value)
		if err != nil {
			return time.Time{}, time.Time{}, err
		}
	}
	duration, err := monitor.WindowDuration(r.URL.Query().Get("window"))
	if err != nil {
		return time.Time{}, time.Time{}, err
	}
	from := to.Add(-duration)
	if value := r.URL.Query().Get("from"); value != "" {
		from, err = time.Parse(time.RFC3339, value)
	}
	return from, to, err
}
