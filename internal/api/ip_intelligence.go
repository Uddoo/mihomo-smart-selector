package api

import (
	"errors"
	"github.com/Uddoo/mihomo-smart-selector/internal/ipintel"
	"mime"
	"net/http"
	"net/url"
	"strings"
)

func (s *Server) WithIPIntelligenceSettings(settings *ipintel.Settings) { s.ipSettings = settings }
func (s *Server) ipIntelligenceSettingsRoute(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Cache-Control", "no-store")
	if s.ipSettings == nil {
		writeError(w, 503, "IP 情报设置暂不可用，请重启服务")
		return
	}
	if origin := r.Header.Get("Origin"); origin != "" {
		u, err := url.Parse(origin)
		if err != nil || !strings.EqualFold(u.Host, r.Host) || (u.Scheme != "http" && u.Scheme != "https") {
			writeError(w, 403, "跨站密钥配置请求被拒绝")
			return
		}
	}
	if r.Header.Get("Sec-Fetch-Site") == "cross-site" {
		writeError(w, 403, "跨站密钥配置请求被拒绝")
		return
	}
	if r.Method == http.MethodGet {
		writeJSON(w, 200, s.ipSettings.State())
		return
	}
	if r.Method != http.MethodPut {
		writeError(w, 405, "method not allowed")
		return
	}
	media, _, err := mime.ParseMediaType(r.Header.Get("Content-Type"))
	if err != nil || media != "application/json" {
		writeError(w, 415, "密钥配置请求必须使用 JSON")
		return
	}
	var payload ipintel.SettingsUpdate
	if !decodeJSON(w, r, &payload) {
		return
	}
	state, err := s.ipSettings.Save(payload)
	if err != nil {
		status := 400
		if errors.Is(err, ipintel.ErrSettingsConflict) {
			status = 409
		}
		writeError(w, status, err.Error())
		return
	}
	writeJSON(w, 200, state)
}
