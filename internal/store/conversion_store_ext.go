package store

import (
	"context"
	"fmt"
	"sync"

	"github.com/ubaas/ubaas/internal/model"
)

// FunnelData stores funnel conversion funnel data for analysis.
type FunnelData struct {
	mu     sync.RWMutex
	goals  map[string]*model.ConversionGoal
	funnel map[string][]model.FunnelStep
}

// NewFunnelData creates a new FunnelData store.
func NewFunnelData() *FunnelData {
	return &FunnelData{
		goals:  make(map[string]*model.ConversionGoal),
		funnel: make(map[string][]model.FunnelStep),
	}
}

// GetGoal retrieves a conversion goal.
func (fd *FunnelData) GetGoal(id string) (*model.ConversionGoal, bool) {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	g, ok := fd.goals[id]
	return g, ok
}

// SetGoal stores a conversion goal.
func (fd *FunnelData) SetGoal(goal *model.ConversionGoal) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.goals[goal.ID] = goal
}

// GetFunnelSteps retrieves funnel steps for a goal.
func (fd *FunnelData) GetFunnelSteps(goalID string) ([]model.FunnelStep, bool) {
	fd.mu.RLock()
	defer fd.mu.RUnlock()
	steps, ok := fd.funnel[goalID]
	return steps, ok
}

// SetFunnelSteps stores funnel steps.
func (fd *FunnelData) SetFunnelSteps(goalID string, steps []model.FunnelStep) {
	fd.mu.Lock()
	defer fd.mu.Unlock()
	fd.funnel[goalID] = steps
}

// ConversionStore provides conversion goal storage.
type ConversionStore struct {
	store *MemoryStore
	fd    *FunnelData
}

// NewConversionStore creates a new ConversionStore.
func NewConversionStore(st *MemoryStore) *ConversionStore {
	return &ConversionStore{
		store: st,
		fd:    NewFunnelData(),
	}
}

// CreateGoal stores a new conversion goal.
func (cs *ConversionStore) CreateGoal(ctx context.Context, goal *model.ConversionGoal) error {
	cs.fd.SetGoal(goal)
	return nil
}

// GetGoal retrieves a conversion goal by ID.
func (cs *ConversionStore) GetGoal(ctx context.Context, goalID string) (*model.ConversionGoal, error) {
	goal, ok := cs.fd.GetGoal(goalID)
	if !ok {
		return nil, fmt.Errorf("conversion goal not found: %s", goalID)
	}
	return goal, nil
}

// UpdateGoal updates an existing conversion goal.
func (cs *ConversionStore) UpdateGoal(ctx context.Context, goal *model.ConversionGoal) error {
	cs.fd.SetGoal(goal)
	return nil
}

// ListGoals returns all conversion goals.
func (cs *ConversionStore) ListGoals(ctx context.Context) ([]*model.ConversionGoal, error) {
	goals := make([]*model.ConversionGoal, 0)
	cs.fd.mu.RLock()
	for _, g := range cs.fd.goals {
		goals = append(goals, g)
	}
	cs.fd.mu.RUnlock()
	return goals, nil
}

// DeleteGoal removes a conversion goal.
func (cs *ConversionStore) DeleteGoal(ctx context.Context, goalID string) error {
	cs.fd.mu.Lock()
	defer cs.fd.mu.Unlock()
	delete(cs.fd.goals, goalID)
	delete(cs.fd.funnel, goalID)
	return nil
}

// SetFunnel stores funnel analysis steps for a goal.
func (cs *ConversionStore) SetFunnel(ctx context.Context, goalID string, steps []model.FunnelStep) error {
	cs.fd.SetFunnelSteps(goalID, steps)
	return nil
}

// GetFunnel retrieves funnel analysis steps.
func (cs *ConversionStore) GetFunnel(ctx context.Context, goalID string) ([]model.FunnelStep, error) {
	steps, ok := cs.fd.GetFunnelSteps(goalID)
	if !ok {
		return nil, fmt.Errorf("funnel data not found for goal: %s", goalID)
	}
	return steps, nil
}
