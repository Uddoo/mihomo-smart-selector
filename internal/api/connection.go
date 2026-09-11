package api

import (
	"errors"
	"mime"
	"net/http"
	"net/url"
	"strings"

	"github.com/Uddoo/mihomo-smart-selector/internal/connection"
)

func (s *Server) WithConnection(manager *connection.Manager) { s.connection = manager }

func (s *Server) connectionRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.connection == nil {
		writeError(w, 503, "连接配置暂不可用，请重启服务后重试")
		return
	}
	// These routes accept credentials and can send them to a new Controller.
	// Reject cross-origin browser requests, including on loopback deployments.
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "http" && u.Scheme != "https") {
			writeError(w, 403, "跨站连接配置请求被拒绝")
			return
		}
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		writeError(w, 403, "跨站连接配置请求被拒绝")
		return
	}
	if r.URL.Path == "/api/v1/connection" && r.Method == http.MethodGet {
		writeJSON(w, 200, s.connection.State())
		return
	}
	if !(r.URL.Path == "/api/v1/connection" && r.Method == http.MethodPut) && !(r.URL.Path == "/api/v1/connection/test" && r.Method == http.MethodPost) {
		writeError(w, 405, "method not allowed")
		return
	}
	mediaType, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || mediaType != "application/json" {
		writeError(w, 415, "连接配置请求必须使用 JSON")
		return
	}
	var payload connection.Update
	if !decodeJSON(w, r, &payload) {
		return
	}
	var result any
	if r.URL.Path == "/api/v1/connection/test" {
		var version string
		version, err = s.connection.Test(r.Context(), payload)
		result = map[string]any{"connected": true, "version": version}
	} else {
		result, err = s.connection.Save(payload)
	}
	if err != nil {
		status := http.StatusBadRequest
		if errors.Is(err, connection.ErrConflict) {
			status = http.StatusConflict
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, 200, result)
}
