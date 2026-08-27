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

	// Store a private copy so callers cannot mutate the stored session
	// through the pointer they passed in.
	stored := cloneSession(session)
	s.sessions[stored.ID] = stored
	s.sessionsByUser[stored.UserID] = append(s.sessionsByUser[stored.UserID], stored)
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
	// Return a snapshot copy so callers can mutate the returned session
	// without disturbing the stored value. This is what allows UpdateSession
	// to reliably compare the incoming (mutated) session against the stored
	// (un-mutated) session.
	return cloneSession(session), nil
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
			results = append(results, cloneSession(session))
		}
	}
	return results, nil
}

// cloneSession returns a deep copy of a session so that the store's internal
// value cannot be mutated by callers of the read methods. The struct value
// copy reproduces all fields (including the unexported version), and the
// Pages slice is copied so appends on the clone do not alias the original.
func cloneSession(s *model.Session) *model.Session {
	if s == nil {
		return nil
	}
	cp := *s
	if s.Pages != nil {
		cp.Pages = make([]string, len(s.Pages))
		copy(cp.Pages, s.Pages)
	}
	return &cp
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

	// A session that has already reached a terminal state (expired or closed)
	// must not be revived with new activity. This is what guards session
	// boundaries: once ExpireSessions (or Complete) has closed a session,
	// appending further events to it would misattribute those events to a
	// session that should be considered finished.
	//
	// Detect a "new activity" update as one that either advances the last
	// event time or bumps the event count beyond what is stored. Such an
	// update can only come from AddEvent/processExistingEvent. Pure
	// metadata updates (e.g. ReclassifyUserType touching only UserType)
	// leave LastEventTime and EventCount unchanged and are still allowed
	// while the session is active.
	if existing.State == model.SessionExpired || existing.State == model.SessionClosed {
		newActivity :=
			session.LastEventTime.After(existing.LastEventTime) ||
				session.EventCount > existing.EventCount
		if newActivity {
			return model.ErrInvalidState
		}
		// No new activity: a terminal session may only be left in its
		// terminal state. Reject any attempt to flip it back to active.
		if session.State != existing.State {
			return model.ErrInvalidState
		}
	}

	// Validate forward state transitions. The table below permits:
	//   active  -> active   (event or metadata updates)
	//   active  -> expired  (ExpireSessions-style transition)
	//   active  -> closed   (Complete)
	//   expired -> expired  (no-op re-expiration, no new activity)
	//   closed  -> closed   (no-op, no new activity)
	// and rejects reviving a terminal session (already handled above) and
	// any unknown state.
	switch existing.State {
	case model.SessionActive:
		switch session.State {
		case model.SessionActive, model.SessionExpired, model.SessionClosed:
		default:
			return model.ErrInvalidState
		}
	case model.SessionExpired:
		if session.State != model.SessionExpired {
			return model.ErrInvalidState
		}
	case model.SessionClosed:
		if session.State != model.SessionClosed {
			return model.ErrInvalidState
		}
	default:
		return model.ErrInvalidState
	}

	// Preserve creation timestamp and ID
	session.CreatedAt = existing.CreatedAt
	session.ID = existing.ID

	// Handle state transitions for metadata consistency
	if session.State == model.SessionExpired {
		session.TotalDuration = existing.ComputeDuration()
	}

	// Commit a defensive copy so the caller's pointer cannot later mutate
	// the stored value (and the Pages slice does not alias the caller's).
	stored := cloneSession(session)
	*existing = *stored
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
		results = append(results, cloneSession(session))
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
