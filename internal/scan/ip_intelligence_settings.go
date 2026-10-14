package scan

import "github.com/Uddoo/mihomo-smart-selector/internal/ipintel"

// Called during runtime construction before workers start.
func (m *Manager) WithIPIntelligenceSettings(s *ipintel.Settings) { m.ipSettings = s }
