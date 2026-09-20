package monitor

import "time"

// A cheap preflight avoids a goroutine, plan clone and contexts on idle ticks.
// step still owns the authoritative selection and cancellation checks.
func (m *TaskRuntime) ready(now time.Time) bool {
	if !m.mu.TryLock() {
		return false
	}
	defer m.mu.Unlock()
	if m.closed || m.plan == nil {
		return false
	}
	// Paused tasks still reconcile provider incidents periodically.
	if !now.Before(m.correlationAt) {
		return true
	}
	if !m.plan.Enabled || m.fault || m.owner.storageFault.Load() {
		return false
	}
	if !now.Before(m.refreshAt) {
		return true
	}
	for i, n := range m.plan.Nodes {
		anchor := nodeAnchor(*m.plan, i)
		if now.Unix() >= anchor {
			slot := (now.Unix() - anchor) / Interval
			last, ok := m.slots[n.ID+"/baseline"]
			if now.Unix()-(anchor+slot*Interval) <= 15 && (!ok || last < slot) {
				return true
			}
		}
		state := m.states[n.ID]
		if m.plan.AutoSwitch && n.Name == m.current && state.Status == "unavailable" && !now.Before(m.failoverAt) {
			return true
		}
		kind := ""
		if m.manual[n.ID] {
			kind = "manual"
		} else if (state.Status == "suspect" || state.Status == "recovering") && now.Sub(state.LastAt) >= 10*time.Second {
			kind = "confirmation"
		} else if n.Name == m.current && now.Sub(state.LastAt) >= 30*time.Second {
			kind = "current"
		}
		if kind != "" {
			if last, ok := m.slots[n.ID+"/"+kind]; !ok || last < now.Unix()/30 {
				return true
			}
		}
	}
	return false
}
