package api

import (
	"net/http"

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
		out, err := s.monitor.Overview(r.Context())
		if err != nil {
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
