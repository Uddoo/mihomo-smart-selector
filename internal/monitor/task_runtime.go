package monitor

import (
	"context"
	"github.com/Uddoo/mihomo-smart-selector/internal/config"
	"github.com/Uddoo/mihomo-smart-selector/internal/history"
	"github.com/Uddoo/mihomo-smart-selector/internal/model"
	"sync"
	"time"
)

type Source interface {
	MonitorScope() string
	MonitorCatalog(context.Context, string, string) (config.ProbeProfile, []model.MonitorNode, string, error)
	MonitorDelay(context.Context, model.MonitorNode, config.Probe) (int, error)
	MonitorReachable(context.Context) bool
}

type TaskRuntime struct {
	owner              *Manager
	saveMu             sync.Mutex
	mu                 sync.Mutex
	dataVersion        uint64
	instanceID         string
	store              *history.Store
	queries            *queryBudget
	source             Source
	plan               *model.MonitorPlan
	profile            config.ProbeProfile
	catalog            map[string]bool
	current, issue     string
	failoverMessage    string
	failoverAt         time.Time
	correlationAt      time.Time
	observedAt         time.Time
	fault              bool
	states             map[string]model.MonitorState
	manual             map[string]bool
	slots              map[string]int64
	refreshAt, cleanAt time.Time
	inflight           context.CancelFunc
	closed             bool
	now                func() time.Time
}

func newTask(owner *Manager, p *model.MonitorPlan) (*TaskRuntime, error) {
	m := &TaskRuntime{owner: owner, store: owner.storeRef, source: owner.sourceRef, plan: p, queries: newQueryBudget(), states: map[string]model.MonitorState{}, manual: map[string]bool{}, slots: map[string]int64{}, instanceID: owner.instance, now: owner.clock}
	m.queries.slots = owner.historySlots
	if p != nil {
		if p.CandidateLimit == 0 {
			p.CandidateLimit = Limits().DefaultCandidateLimit
		}
		var err error
		m.states, m.slots, err = m.store.MonitorCheckpoint(context.Background(), p.Nodes)
		if err != nil {
			return nil, err
		}
		owner.budget.set(p.TaskID, requestBudget(len(p.Nodes), 1), p.Enabled)
	}
	return m, nil
}
