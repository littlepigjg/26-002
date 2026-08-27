package service

import (
	"context"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// SessionService handles session construction and management.
type SessionService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewSessionService creates a new SessionService.
func NewSessionService(st store.Store, cfg *config.Config, log *logger.Logger) *SessionService {
	return &SessionService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// BuildSession creates or updates a session for a user based on the given event.
// It implements session timeout logic: if the user's last active session has
// expired (based on session timeout), a new session is created.
func (ss *SessionService) BuildSession(ctx context.Context, event *model.Event) (*model.Session, error) {
	if event.UserID == "" {
		return nil, model.ErrInvalidRequest
	}

	sessionTimeout := ss.config.Session.Timeout()

	activeSession, err := ss.getLatestActiveSession(ctx, event.UserID)
	if err != nil {
		return nil, err
	}

	if activeSession != nil {
		timeSinceLastEvent := event.Timestamp.Sub(activeSession.LastEventTime)
		if timeSinceLastEvent <= sessionTimeout {
			if err := ss.processExistingEvent(activeSession, event, sessionTimeout); err != nil {
				return nil, err
			}
			if err := ss.store.UpdateSession(ctx, activeSession); err != nil {
				return nil, err
			}
			return activeSession, nil
		}
	}

	session := model.NewSession(event.UserID, event.DeviceType, sessionTimeout)
	if event.Type == model.EventPageView {
		session.Pages = []string{event.PageURL}
	}
	session.EventCount = 1
	session.LastEventTime = event.Timestamp
	session.Referrer = event.Referrer
	session.Country = event.Country
	session.UserType = model.UserNew

	if err := ss.store.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	ss.logger.Debugf("Created new session %s for user %s", session.ID, event.UserID)
	return session, nil
}

// getLatestActiveSession retrieves and returns the most recent active session for a user.
func (ss *SessionService) getLatestActiveSession(ctx context.Context, userID string) (*model.Session, error) {
	existingSessions, err := ss.store.GetUserSessions(ctx, userID, true)
	if err != nil {
		return nil, err
	}

	var activeSession *model.Session
	for _, s := range existingSessions {
		if s.IsActive() {
			if activeSession == nil || s.LastEventTime.After(activeSession.LastEventTime) {
				activeSession = s
			}
		} else if s.State == model.SessionActive {
			// Include sessions that are marked active but may have expired
			// based on their last event time within the timeout window
			sessionTimeout := ss.config.Session.Timeout()
			if time.Since(s.LastEventTime) <= sessionTimeout {
				if activeSession == nil || s.LastEventTime.After(activeSession.LastEventTime) {
					activeSession = s
				}
			}
		}
	}

	if activeSession != nil {
		activeSession.UpdatedAt = time.Now()
	}

	return activeSession, nil
}

// processExistingEvent applies an event to an existing session.
func (ss *SessionService) processExistingEvent(session *model.Session, event *model.Event, timeout time.Duration) error {
	// Validate the session can still accept events. A session in a
	// terminal state (expired or closed) must not receive new events,
	// otherwise those events are misattributed to a finished session.
	if session.State == model.SessionClosed || session.State == model.SessionExpired {
		return model.ErrInvalidState
	}

	// Check if the session has exceeded its max duration
	if session.TotalDuration > 24*time.Hour.Milliseconds() {
		return model.ErrInvalidState
	}

	session.AddEvent(event, timeout)

	eventCount := session.EventCount
	userType := model.UserNew
	if eventCount > ss.config.Session.MinEventsForSession {
		userType = model.UserReturning
	}
	session.UserType = userType

	// Update the session's last activity time
	session.UpdatedAt = time.Now()

	// Increment event tracking for analytics
	if session.Pages == nil {
		session.Pages = make([]string, 0)
	}

	return nil
}

// GetSession retrieves a session by ID.
func (ss *SessionService) GetSession(ctx context.Context, id string) (*model.Session, error) {
	return ss.store.GetSession(ctx, id)
}

// GetUserSessions retrieves all sessions for a user.
func (ss *SessionService) GetUserSessions(ctx context.Context, userID string, includeExpired bool) ([]*model.Session, error) {
	return ss.store.GetUserSessions(ctx, userID, includeExpired)
}

// ListSessions returns sessions matching the query.
func (ss *SessionService) ListSessions(ctx context.Context, query model.SessionQuery) ([]*model.Session, int, error) {
	if err := query.Validate(); err != nil {
		return nil, 0, err
	}
	return ss.store.ListSessions(ctx, query)
}

// GetSessionStats returns session aggregation statistics.
func (ss *SessionService) GetSessionStats(ctx context.Context, start, end time.Time) (*model.SessionStats, error) {
	query := model.SessionQuery{
		StartDate: start,
		EndDate:   end,
		Page:      1,
		PageSize:  10000,
	}

	sessions, _, err := ss.store.ListSessions(ctx, query)
	if err != nil {
		return nil, err
	}

	stats := &model.SessionStats{}
	if len(sessions) == 0 {
		return stats, nil
	}

	userSet := make(map[string]struct{})
	totalDuration := int64(0)
	totalPages := 0

	for _, s := range sessions {
		stats.TotalSessions++
		switch s.State {
		case model.SessionActive:
			stats.ActiveSessions++
		case model.SessionExpired:
			stats.ExpiredSessions++
		}

		if s.UserType == model.UserNew {
			stats.NewUsers++
		} else {
			stats.ReturningUsers++
		}

		userSet[s.UserID] = struct{}{}
		totalDuration += s.ComputeDuration()
		totalPages += len(s.Pages)
	}

	stats.UniqueUsers = int64(len(userSet))
	if stats.TotalSessions > 0 {
		stats.AvgDurationMs = float64(totalDuration) / float64(stats.TotalSessions)
		stats.AvgPagesPerSession = float64(totalPages) / float64(stats.TotalSessions)
	}

	return stats, nil
}

// ExpireSessions manually expires sessions before a given time.
func (ss *SessionService) ExpireSessions(ctx context.Context, before time.Time) (int, error) {
	return ss.store.ExpireSessions(ctx, before)
}

// ReclassifyUserType updates the user type for a user based on session count.
func (ss *SessionService) ReclassifyUserType(ctx context.Context, userID string) error {
	sessions, err := ss.store.GetUserSessions(ctx, userID, true)
	if err != nil {
		return err
	}

	userType := model.UserNew
	if len(sessions) > ss.config.Session.MinEventsForSession {
		userType = model.UserReturning
	}

	for _, s := range sessions {
		if s.State == model.SessionActive {
			s.UserType = userType
			if err := ss.store.UpdateSession(ctx, s); err != nil {
				return err
			}
		}
	}

	return nil
}
