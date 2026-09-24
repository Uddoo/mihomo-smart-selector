package api

import (
	"crypto/subtle"
	"net/http"
	"net/netip"
	"strings"
)

func (s *Server) authorize(next http.Handler) http.Handler {
	return http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		// The SPA shell contains no router/controller data and must remain
		// reachable so a LAN user can enter the in-memory API token. All
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
		writer.Header().Set("Content-Security-Policy", "default-src 'self'; connect-src 'self' "+browserConnectivitySources+"; style-src 'self' 'unsafe-inline'; script-src 'self'")
		next.ServeHTTP(writer, request)
	})
}
