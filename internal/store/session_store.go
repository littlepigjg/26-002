package store

import (
	"context"
	"time"

	"github.com/ubaas/ubaas/internal/model"
)

// CreateSession stores a new session.
func (s *MemoryStore) CreateSession(ctx context.Context, session *model.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	s.sessions[session.ID] = session
	s.sessionsByUser[session.UserID] = append(s.sessionsByUser[session.UserID], session)
	return nil
}

// GetSession retrieves a session by ID.
func (s *MemoryStore) GetSession(ctx context.Context, id string) (*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	session, ok := s.sessions[id]
	if !ok {
		return nil, model.ErrSessionNotFound
	}
	return session, nil
}

// GetUserSessions retrieves all sessions for a user.
func (s *MemoryStore) GetUserSessions(ctx context.Context, userID string, includeExpired bool) ([]*model.Session, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userSessions := s.sessionsByUser[userID]
	if len(userSessions) == 0 {
		return []*model.Session{}, nil
	}

	var results []*model.Session
	for _, session := range userSessions {
		if includeExpired || session.IsActive() {
			results = append(results, session)
		}
	}
	return results, nil
}

// UpdateSession updates an existing session.
func (s *MemoryStore) UpdateSession(ctx context.Context, session *model.Session) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	existing, ok := s.sessions[session.ID]
	if !ok {
		return model.ErrSessionNotFound
	}

	*existing = *session
	return nil
}

// ListSessions returns sessions matching the query with pagination.
func (s *MemoryStore) ListSessions(ctx context.Context, query model.SessionQuery) ([]*model.Session, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isOpen {
		return nil, 0, model.ErrStoreClosed
	}

	var results []*model.Session
	for _, session := range s.sessions {
		if query.UserID != "" && session.UserID != query.UserID {
			continue
		}
		if query.State != "" && session.State != query.State {
			continue
		}
		if query.UserType != "" && session.UserType != query.UserType {
			continue
		}
		if query.DeviceType != "" && session.DeviceType != query.DeviceType {
			continue
		}
		if !query.StartDate.IsZero() && session.StartTime.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && session.StartTime.After(query.EndDate) {
			continue
		}
		if query.MinDurationMs > 0 {
			duration := session.ComputeDuration()
			if duration < query.MinDurationMs {
				continue
			}
		}
		results = append(results, session)
	}

	total := len(results)

	// Apply pagination
	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return []*model.Session{}, total, nil
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

// ActiveSessionCount returns the number of active sessions.
func (s *MemoryStore) ActiveSessionCount(ctx context.Context) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, session := range s.sessions {
		if session.IsActive() {
			count++
		}
	}
	return count, nil
}

// ExpireSessions expires all sessions before the given time.
func (s *MemoryStore) ExpireSessions(ctx context.Context, before time.Time) (int, error) {
	s.mu.Lock()
	defer s.mu.Unlock()

	count := 0
	for _, session := range s.sessions {
		if session.State == model.SessionActive && session.EndTime.Before(before) {
			session.State = model.SessionExpired
			count++
		}
	}
	return count, nil
}
