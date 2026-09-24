package api

import (
	"embed"
	"fmt"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/connection"
	"github.com/Uddoo/mihomo-smart-selector/internal/mihomo"
	"github.com/Uddoo/mihomo-smart-selector/internal/monitor"
	"github.com/Uddoo/mihomo-smart-selector/internal/scan"
	"io/fs"
	"net"
	"net/http"
	"net/netip"
)

//go:embed static/*
var embeddedStatic embed.FS

type Server struct {
	config     config.HTTPConfig
	manager    *scan.Manager
	controller mihomo.Client
	static     http.Handler
	allowed    []netip.Prefix
	exposed    bool
	monitor    *monitor.Manager
	connection *connection.Manager
	service    *serviceControl
}

func New(cfg config.HTTPConfig, manager *scan.Manager, controller mihomo.Client) (*Server, error) {
	staticFiles, err := fs.Sub(embeddedStatic, "static")
	if err != nil {
		return nil, fmt.Errorf("open embedded frontend: %w", err)
	}
	staticHandler, err := newStaticHandler(staticFiles)
	if err != nil {
		return nil, fmt.Errorf("prepare embedded frontend: %w", err)
	}
	host, _, err := net.SplitHostPort(cfg.Listen)
	if err != nil {
		return nil, fmt.Errorf("parse HTTP listener: %w", err)
	}
	exposed := host != "localhost"
	if ip, parseErr := netip.ParseAddr(host); parseErr == nil {
		exposed = !ip.IsLoopback()
	}
	server := &Server{
		config: cfg, manager: manager, controller: controller,
		static: staticHandler, exposed: exposed,
	}
	for _, cidr := range cfg.AllowedCIDRs {
		prefix, err := netip.ParsePrefix(cidr)
		if err != nil {
			return nil, fmt.Errorf("parse allowed CIDR: %w", err)
		}
		server.allowed = append(server.allowed, prefix)
	}
	return server, nil
}

func (s *Server) Handler() http.Handler {
	return s.securityHeaders(s.authorize(http.HandlerFunc(s.route)))
}
