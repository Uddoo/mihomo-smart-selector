package api

import (
	"mime"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"sync/atomic"
)

type serviceControl struct {
	mu         sync.Mutex
	pending    atomic.Bool
	instanceID string
	version    string
	prepare    func() error
	restart    func()
}

func (s *Server) WithServiceControl(instanceID, version string, prepare func() error, restart func()) {
	s.service = &serviceControl{instanceID: instanceID, version: version, prepare: prepare, restart: restart}
}

func (s *Server) serviceRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.service == nil {
		writeError(w, http.StatusNotImplemented, "此实例不支持页面重启，请在启动终端中重启服务")
		return
	}
	control := s.service
	if r.URL.Path == "/api/v1/service" && r.Method == http.MethodGet {
		status := "running"
		if control.pending.Load() {
			status = "restarting"
		}
		active := 0
		if s.manager != nil {
			active = s.manager.ActiveScans()
		}
		writeJSON(w, http.StatusOK, map[string]any{"instance_id": control.instanceID, "version": control.version, "status": status, "active_scans": active})
		return
	}
	if r.URL.Path != "/api/v1/service/restart" || r.Method != http.MethodPost {
		writeError(w, http.StatusMethodNotAllowed, "method not allowed")
		return
	}
	// A browser cannot restart a loopback service with a cross-site form/fetch.
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "http" && u.Scheme != "https") {
			writeError(w, http.StatusForbidden, "跨站重启请求被拒绝")
			return
		}
	}
	if site := r.Header.Get("Sec-Fetch-Site"); site == "cross-site" {
		writeError(w, http.StatusForbidden, "跨站重启请求被拒绝")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, http.StatusUnsupportedMediaType, "重启请求必须使用 JSON")
		return
	}
	var payload struct {
		InstanceID string `json:"instance_id"`
		Confirm    bool   `json:"confirm"`
	}
	if !decodeJSON(w, r, &payload) {
		return
	}
	if !payload.Confirm {
		writeError(w, http.StatusBadRequest, "请确认重启服务")
		return
	}
	control.mu.Lock()
	defer control.mu.Unlock()
	if payload.InstanceID != control.instanceID {
		writeError(w, http.StatusConflict, "服务实例已更新，请重新加载页面后操作")
		return
	}
	if control.pending.Load() {
		writeJSON(w, http.StatusAccepted, map[string]string{"status": "restarting", "instance_id": control.instanceID})
		return
	}
	if err := control.prepare(); err != nil {
		writeError(w, http.StatusConflict, err.Error())
		return
	}
	control.pending.Store(true)
	writeJSON(w, http.StatusAccepted, map[string]string{"status": "restarting", "instance_id": control.instanceID})
	_ = http.NewResponseController(w).Flush()
	control.restart()
}
