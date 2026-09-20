package monitor

import (
	"sync"
	"time"
)

const BackgroundWorkers = 2
const MaxRequestsPerMinute = 360

type budgetAttempt struct {
	task string
	at   time.Time
}

// Both the rolling Controller budget and proportional task shares account for
// every probe kind. Edits, pauses and resumes never erase spent requests.
type requestLimiter struct {
	mu       sync.Mutex
	weights  map[string]int
	attempts []budgetAttempt
}

func (b *requestLimiter) set(task string, weight int, enabled bool) {
	b.mu.Lock()
	defer b.mu.Unlock()
	if enabled {
		b.weights[task] = weight
	} else {
		delete(b.weights, task)
	}
}

func (b *requestLimiter) take(task string, now time.Time) bool {
	b.mu.Lock()
	defer b.mu.Unlock()
	recent := b.attempts[:0]
	used := 0
	for _, a := range b.attempts {
		if now.Sub(a.at) < time.Minute {
			recent = append(recent, a)
			if a.task == task {
				used++
			}
		}
	}
	b.attempts = recent
	total := 0
	for _, weight := range b.weights {
		total += weight
	}
	weight := b.weights[task]
	if weight == 0 || total == 0 {
		return false
	}
	limit := min(total, MaxRequestsPerMinute)
	share := weight
	if total > limit {
		share = max(1, weight*limit/total)
	}
	if len(recent) >= limit || used >= share {
		return false
	}
	b.attempts = append(b.attempts, budgetAttempt{task, now})
	return true
}

func (b *requestLimiter) snapshot(now time.Time) (limit, used int) {
	b.mu.Lock()
	defer b.mu.Unlock()
	for _, weight := range b.weights {
		limit += weight
	}
	limit = min(limit, MaxRequestsPerMinute)
	for _, a := range b.attempts {
		if now.Sub(a.at) < time.Minute {
			used++
		}
	}
	return
}
