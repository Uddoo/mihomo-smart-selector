package history

// EvidenceVersion changes only for baseline evidence or retention maintenance.
// Versions are process-local; overview cache entries also have a bounded TTL.
type EvidenceVersion struct{ Epoch, Series uint64 }

func (s *Store) MonitorEvidenceVersion(series string) EvidenceVersion {
	s.evidenceMu.Lock()
	defer s.evidenceMu.Unlock()
	return EvidenceVersion{s.evidenceEpoch.Load(), s.evidenceVersions[series]}
}

func (s *Store) baselineChanged(series string) {
	s.evidenceMu.Lock()
	defer s.evidenceMu.Unlock()
	if s.evidenceVersions == nil {
		s.evidenceVersions = map[string]uint64{}
	}
	s.evidenceVersions[series]++
}
