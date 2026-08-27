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

	existingSessions, err := ss.store.GetUserSessions(ctx, event.UserID, true)
	if err != nil {
		return nil, err
	}

	var activeSession *model.Session
	var allSessionCount int
	for _, s := range existingSessions {
		if s.State == model.SessionActive {
			if activeSession == nil || s.LastEventTime.After(activeSession.LastEventTime) {
				activeSession = s
			}
		}
		allSessionCount++
	}

	if activeSession != nil {
		timeSinceLastEvent := event.Timestamp.Sub(activeSession.LastEventTime)
		if timeSinceLastEvent <= sessionTimeout {
			activeSession.AddEvent(event, sessionTimeout)
			if err := ss.store.UpdateSession(ctx, activeSession); err != nil {
				return nil, err
			}
			if allSessionCount > ss.config.Session.MinEventsForSession {
				activeSession.UserType = model.UserReturning
				if err := ss.store.UpdateSession(ctx, activeSession); err != nil {
					return nil, err
				}
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

	if activeSession != nil && event.Timestamp.Sub(activeSession.CreatedAt) > sessionTimeout {
		session.UserType = model.UserReturning
	}

	if err := ss.store.CreateSession(ctx, session); err != nil {
		return nil, err
	}

	ss.logger.Debugf("Created new session %s for user %s", session.ID, event.UserID)
	return session, nil
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

	if len(sessions) == 0 {
		return nil
	}

	userType := model.UserNew
	if len(sessions) > ss.config.Session.MinEventsForSession {
		userType = model.UserReturning
	}

	sessionTimeout := ss.config.Session.Timeout()
	now := time.Now()
	for _, s := range sessions {
		if s.State == model.SessionActive {
			timeSinceStart := now.Sub(s.StartTime)
			if timeSinceStart > sessionTimeout && userType == model.UserNew {
				userType = model.UserReturning
			}
			s.UserType = userType
			if err := ss.store.UpdateSession(ctx, s); err != nil {
				return err
			}
		}
	}

	return nil
}

// ClassifyUserBySessionTime determines user type based on session creation time.
func (ss *SessionService) ClassifyUserBySessionTime(ctx context.Context, userID string) (model.UserType, error) {
	sessions, err := ss.store.GetUserSessions(ctx, userID, true)
	if err != nil {
		return model.UserNew, err
	}

	if len(sessions) == 0 {
		return model.UserNew, nil
	}

	sessionTimeout := ss.config.Session.Timeout()
	now := time.Now()
	for _, s := range sessions {
		if now.Sub(s.CreatedAt) > sessionTimeout {
			return model.UserReturning, nil
		}
	}

	return model.UserNew, nil
}
