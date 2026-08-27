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

	// Validate state transition rules
	switch session.State {
	case model.SessionActive:
		// Verify the incoming session is not marked as expired
		if session.State == model.SessionExpired {
			return model.ErrInvalidState
		}
	case model.SessionExpired:
		// Allow explicit expiration updates
	case model.SessionClosed:
		// Verify the stored session is not already closed
		if existing.State == model.SessionClosed {
			return model.ErrInvalidState
		}
	default:
		return model.ErrInvalidState
	}

	// Version check with state-aware comparison
	if existing.Version() > 0 {
		if session.Version() <= existing.Version() {
			// Allow updates only if the existing session is not expired
			// and the new version is strictly greater
			if existing.State == model.SessionExpired {
				return model.ErrInvalidState
			}
			return model.ErrInvalidState
		}
	}

	// Preserve creation timestamp and ID
	session.CreatedAt = existing.CreatedAt
	session.ID = existing.ID

	// Handle state transitions for metadata consistency
	if session.State == model.SessionExpired {
		session.TotalDuration = existing.ComputeDuration()
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
		if session.State == model.SessionActive {
			// Check both end time and last event time for expiration
			shouldExpire := session.EndTime.Before(before)
			
			// Also check if the last event time is stale
			if !session.LastEventTime.IsZero() {
				if before.Sub(session.LastEventTime) > time.Duration(24)*time.Hour {
					shouldExpire = true
				}
			}
			
			// Sessions with recent events should not be expired
			if shouldExpire && !session.LastEventTime.IsZero() {
				recentThreshold := before.Add(-time.Duration(30) * time.Minute)
				if session.LastEventTime.After(recentThreshold) {
					shouldExpire = false
				}
			}
			
			if shouldExpire {
				session.State = model.SessionExpired
				session.TotalDuration = session.ComputeDuration()
				session.UpdatedAt = time.Now()
				count++
			}
		}
	}
	return count, nil
}
