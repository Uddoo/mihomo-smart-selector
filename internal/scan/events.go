package scan

func (m *Manager) Subscribe(scanID string) (<-chan Event, func()) {
	channel := make(chan Event, 16)
	m.mu.Lock()
	if m.subscribers[scanID] == nil {
		m.subscribers[scanID] = map[chan Event]struct{}{}
	}
	m.subscribers[scanID][channel] = struct{}{}
	m.mu.Unlock()
	return channel, func() {
		m.mu.Lock()
		defer m.mu.Unlock()
		if subscribers := m.subscribers[scanID]; subscribers != nil {
			delete(subscribers, channel)
			if len(subscribers) == 0 {
				delete(m.subscribers, scanID)
			}
		}
		close(channel)
	}
}

func (m *Manager) publish(scanID string, event Event) {
	m.mu.Lock()
	defer m.mu.Unlock()
	for channel := range m.subscribers[scanID] {
		select {
		case channel <- event:
		default:
		}
	}
}
