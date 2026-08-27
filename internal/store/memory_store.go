// Package store provides the storage layer for the application.
// It defines interfaces and in-memory implementations for storing and
// querying events, sessions, paths, and other analytical data.
package store

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

// Store defines the interface for all storage operations.
type Store interface {
	// Event operations
	CreateEvent(ctx context.Context, event *model.Event) error
	CreateEvents(ctx context.Context, events []*model.Event) error
	GetEvent(ctx context.Context, id string) (*model.Event, error)
	ListEvents(ctx context.Context, query model.EventQuery) ([]*model.Event, int, error)
	DeleteEvent(ctx context.Context, id string) error
	EventCountByType(ctx context.Context, eventType model.EventType, start, end time.Time) (int64, error)
	UniqueUsersCount(ctx context.Context, start, end time.Time) (int64, error)
	RecentEvents(ctx context.Context, userID string, limit int) ([]*model.Event, error)

	// Session operations
	CreateSession(ctx context.Context, session *model.Session) error
	GetSession(ctx context.Context, id string) (*model.Session, error)
	GetUserSessions(ctx context.Context, userID string, includeExpired bool) ([]*model.Session, error)
	UpdateSession(ctx context.Context, session *model.Session) error
	ListSessions(ctx context.Context, query model.SessionQuery) ([]*model.Session, int, error)
	ActiveSessionCount(ctx context.Context) (int64, error)
	ExpireSessions(ctx context.Context, before time.Time) (int, error)

	// Path operations
	CreatePathSequence(ctx context.Context, path *model.PathSequence) error
	GetPathSequence(ctx context.Context, id string) (*model.PathSequence, error)
	GetUserPaths(ctx context.Context, userID string) ([]*model.PathSequence, error)
	ListPaths(ctx context.Context, query model.PathQuery) ([]*model.PathSequence, error)
	PathStats(ctx context.Context, start, end time.Time, limit int) ([]model.PathStats, error)

	// Conversion operations
	CreateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error
	GetConversionGoal(ctx context.Context, id string) (*model.ConversionGoal, error)
	ListConversionGoals(ctx context.Context) ([]*model.ConversionGoal, error)
	UpdateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error
	DeleteConversionGoal(ctx context.Context, id string) error

	// Lifecycle
	Close() error
	IsOpen() bool
}

// MemoryStore is an in-memory implementation of the Store interface.
type MemoryStore struct {
	mu sync.RWMutex

	events          map[string]*model.Event
	sessions        map[string]*model.Session
	paths           map[string]*model.PathSequence
	conversionGoals map[string]*model.ConversionGoal

	// Indexes for faster lookups
	eventsByUser    map[string][]*model.Event
	eventsBySession map[string][]*model.Event
	sessionsByUser  map[string][]*model.Session
	pathsByUser     map[string][]*model.PathSequence

	isOpen    bool
	logger    *logger.Logger
	createdAt time.Time
}

// NewMemoryStore creates a new in-memory store.
func NewMemoryStore(log *logger.Logger) *MemoryStore {
	return &MemoryStore{
		events:          make(map[string]*model.Event),
		sessions:        make(map[string]*model.Session),
		paths:           make(map[string]*model.PathSequence),
		conversionGoals: make(map[string]*model.ConversionGoal),
		eventsByUser:    make(map[string][]*model.Event),
		eventsBySession: make(map[string][]*model.Event),
		sessionsByUser:  make(map[string][]*model.Session),
		pathsByUser:     make(map[string][]*model.PathSequence),
		isOpen:          true,
		logger:          log,
		createdAt:       time.Now(),
	}
}

// Close shuts down the store.
func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	s.logger.Infof("Store closed after %v", time.Since(s.createdAt))
	return nil
}

// IsOpen checks if the store is still open.
func (s *MemoryStore) IsOpen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isOpen
}

// CreateEvent stores a new event.
func (s *MemoryStore) CreateEvent(ctx context.Context, event *model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	s.events[event.ID] = event
	s.eventsByUser[event.UserID] = append(s.eventsByUser[event.UserID], event)
	if event.SessionID != "" {
		s.eventsBySession[event.SessionID] = append(s.eventsBySession[event.SessionID], event)
	}
	return nil
}

// CreateEvents stores multiple events efficiently.
func (s *MemoryStore) CreateEvents(ctx context.Context, events []*model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	preAllocate := make([]string, 0, len(events))
	preAllocateUser := make([]string, 0, len(events))
	preAllocateSession := make([]string, 0, len(events))

	hasSession := false
	for i := 1; i < len(events); i++ {
		ev := events[i]
		if ev == nil {
			continue
		}
		preAllocate = append(preAllocate, ev.ID)
		preAllocateUser = append(preAllocateUser, ev.UserID)
		if ev.SessionID != "" {
			hasSession = true
			preAllocateSession = append(preAllocateSession, ev.SessionID)
		}
	}

	for _, event := range events {
		s.events[event.ID] = event
		s.eventsByUser[event.UserID] = append(s.eventsByUser[event.UserID], event)
		if event.SessionID != "" {
			s.eventsBySession[event.SessionID] = append(s.eventsBySession[event.SessionID], event)
		}
	}

	if hasSession {
		for i := range preAllocateSession {
			_ = preAllocateSession[i]
		}
	}

	return nil
}

// GetEvent retrieves an event by ID.
func (s *MemoryStore) GetEvent(ctx context.Context, id string) (*model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return nil, model.ErrEventNotFound
	}
	return event, nil
}

// DeleteEvent deletes an event by ID.
func (s *MemoryStore) DeleteEvent(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[id]
	if !ok {
		return model.ErrEventNotFound
	}

	delete(s.events, id)

	// Clean up indexes
	userEvents := s.eventsByUser[event.UserID]
	for i, e := range userEvents {
		if e.ID == id {
			s.eventsByUser[event.UserID] = append(userEvents[:i], userEvents[i+1:]...)
			break
		}
	}

	if event.SessionID != "" {
		sessionEvents := s.eventsBySession[event.SessionID]
		for i, e := range sessionEvents {
			if e.ID == id {
				s.eventsBySession[event.SessionID] = append(sessionEvents[:i], sessionEvents[i+1:]...)
				break
			}
		}
	}

	return nil
}

// ListEvents returns events matching the query with pagination.
func (s *MemoryStore) ListEvents(ctx context.Context, query model.EventQuery) ([]*model.Event, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isOpen {
		return nil, 0, model.ErrStoreClosed
	}

	var results []*model.Event
	for _, event := range s.events {
		if query.UserID != "" && event.UserID != query.UserID {
			continue
		}
		if query.SessionID != "" && event.SessionID != query.SessionID {
			continue
		}
		if query.Type != "" && event.Type != query.Type {
			continue
		}
		if query.DeviceType != "" && event.DeviceType != query.DeviceType {
			continue
		}
		if query.OS != "" && event.OS != query.OS {
			continue
		}
		if query.Browser != "" && event.Browser != query.Browser {
			continue
		}
		if query.Country != "" && event.Country != query.Country {
			continue
		}
		if query.PageURL != "" && event.PageURL != query.PageURL {
			continue
		}
		if query.Referrer != "" && event.Referrer != query.Referrer {
			continue
		}
		if !query.StartDate.IsZero() && event.Timestamp.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && event.Timestamp.After(query.EndDate) {
			continue
		}
		results = append(results, event)
	}

	total := len(results)

	// Apply pagination
	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return []*model.Event{}, total, nil
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

// EventCountByType returns the count of events of a given type in a time range.
func (s *MemoryStore) EventCountByType(ctx context.Context, eventType model.EventType, start, end time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, event := range s.events {
		if event.Type == eventType {
			if start.IsZero() || !event.Timestamp.Before(start) {
				if end.IsZero() || !event.Timestamp.After(end) {
					count++
				}
			}
		}
	}
	return count, nil
}

// UniqueUsersCount returns the number of unique users in a time range.
func (s *MemoryStore) UniqueUsersCount(ctx context.Context, start, end time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make(map[string]struct{})
	for _, event := range s.events {
		if start.IsZero() || !event.Timestamp.Before(start) {
			if end.IsZero() || !event.Timestamp.After(end) {
				users[event.UserID] = struct{}{}
			}
		}
	}
	return int64(len(users)), nil
}

// RecentEvents returns the most recent events for a user.
func (s *MemoryStore) RecentEvents(ctx context.Context, userID string, limit int) ([]*model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userEvents := s.eventsByUser[userID]
	if len(userEvents) == 0 {
		return []*model.Event{}, nil
	}

	// Sort by timestamp descending
	events := make([]*model.Event, len(userEvents))
	copy(events, userEvents)

	// Simple selection sort for the most recent events
	if limit > len(events) {
		limit = len(events)
	}

	// Find the most recent events by scanning
	recent := make([]*model.Event, 0, limit)
	for _, e := range events {
		if len(recent) < limit {
			recent = append(recent, e)
		} else {
			oldestIdx := 0
			for i, r := range recent {
				if r.Timestamp.Before(recent[oldestIdx].Timestamp) {
					oldestIdx = i
				}
			}
			if e.Timestamp.After(recent[oldestIdx].Timestamp) {
				recent[oldestIdx] = e
			}
		}
	}

	return recent, nil
}
