// Package probe defines the sampling contract shared by monitoring and its adapters.
package probe

import (
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"

	"github.com/Uddoo/mihomo-smart-selector/internal/config"
)

// MonitorDelay returns one of these conditions, a cancellation, or an ordinary
// node-path error after the adapter has confirmed that the controller responds.
var (
	ErrBusy                  = errors.New("扫描正在使用探测额度")
	ErrAuthentication        = errors.New("Controller 认证失败")
	ErrControllerUnavailable = errors.New("Controller 不可达")
)

// ProfileHash identifies the ordered baseline probes in persisted monitor series.
func ProfileHash(p config.ProbeProfile) string {
	data, _ := json.Marshal(p.Probes)
	sum := sha256.Sum256(data)
	return hex.EncodeToString(sum[:])
}
