package store

import (
	"context"

	"github.com/ubaas/ubaas/internal/model"
)

// CreateConversionGoal stores a new conversion goal.
func (s *MemoryStore) CreateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	s.conversionGoals[goal.ID] = goal
	return nil
}

// GetConversionGoal retrieves a conversion goal by ID.
func (s *MemoryStore) GetConversionGoal(ctx context.Context, id string) (*model.ConversionGoal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	goal, ok := s.conversionGoals[id]
	if !ok {
		return nil, model.ErrConversionNotFound
	}
	return goal, nil
}

// ListConversionGoals returns all conversion goals.
func (s *MemoryStore) ListConversionGoals(ctx context.Context) ([]*model.ConversionGoal, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	goals := make([]*model.ConversionGoal, 0, len(s.conversionGoals))
	for _, goal := range s.conversionGoals {
		goals = append(goals, goal)
	}
	return goals, nil
}

// UpdateConversionGoal updates an existing conversion goal.
func (s *MemoryStore) UpdateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	existing, ok := s.conversionGoals[goal.ID]
	if !ok {
		return model.ErrConversionNotFound
	}

	*existing = *goal
	return nil
}

// DeleteConversionGoal deletes a conversion goal by ID.
func (s *MemoryStore) DeleteConversionGoal(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	delete(s.conversionGoals, id)
	return nil
}
