package mihomo

import (
	"context"
	"fmt"
	"net"
	"net/netip"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

var errControllerTarget = fmt.Errorf("Controller 目标必须是本机或私网地址；其他目标请在 YAML 中配置")

func localControllerAddress(address netip.Addr) bool {
	address = address.Unmap()
	// The EC2 IPv6 metadata endpoint is inside ULA space.
	return address.Zone() == "" && address != netip.MustParseAddr("fd00:ec2::254") && (address.IsLoopback() || address.IsPrivate())
}

// ValidateConnectionTarget performs offline validation, so saving a connection
// does not require a live Controller. Hostnames are checked again at dial time.
// configuredController must come from operator-owned YAML, never a saved draft.
func ValidateConnectionTarget(controller, configuredController string) error {
	u, err := config.ParseControllerURL(controller)
	if err != nil {
		return err
	}
	if config.SameController(controller, configuredController) {
		return nil
	}
	if address, err := netip.ParseAddr(u.Hostname()); err == nil && !localControllerAddress(address) {
		return errControllerTarget
	}
	return nil
}

type controllerDialer struct {
	trusted bool
	lookup  func(context.Context, string, string) ([]netip.Addr, error)
	dial    func(context.Context, string, string) (net.Conn, error)
}

func (d controllerDialer) DialContext(ctx context.Context, network, address string) (net.Conn, error) {
	host, port, err := net.SplitHostPort(address)
	if err != nil {
		return nil, errControllerTarget
	}
	addresses, err := d.lookup(ctx, "ip", host)
	if err != nil || len(addresses) == 0 {
		return nil, fmt.Errorf("无法解析 Controller 地址")
	}
	// Validate every answer before connecting, then dial the checked IP directly.
	// A second DNS lookup or an environment proxy would bypass this boundary.
	for _, ip := range addresses {
		if !ip.IsValid() || (!d.trusted && !localControllerAddress(ip)) {
			return nil, errControllerTarget
		}
	}
	for _, ip := range addresses {
		var conn net.Conn
		conn, err = d.dial(ctx, network, net.JoinHostPort(ip.String(), port))
		if err == nil {
			return conn, nil
		}
	}
	return nil, err
}
