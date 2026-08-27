package model

import (
	"time"
)

// DeviceType represents the type of device used by the user.
type DeviceType string

const (
	// DeviceDesktop represents a desktop/laptop device.
	DeviceDesktop DeviceType = "desktop"
	// DeviceMobile represents a mobile phone device.
	DeviceMobile DeviceType = "mobile"
	// DeviceTablet represents a tablet device.
	DeviceTablet DeviceType = "tablet"
	// DeviceOther represents an unknown/other device.
	DeviceOther DeviceType = "other"
)

// Valid checks if the DeviceType is valid.
func (dt DeviceType) Valid() bool {
	switch dt {
	case DeviceDesktop, DeviceMobile, DeviceTablet, DeviceOther:
		return true
	default:
		return false
	}
}

// UserType represents whether the user is new or returning.
type UserType string

const (
	// UserNew represents a new user.
	UserNew UserType = "new"
	// UserReturning represents a returning user.
	UserReturning UserType = "returning"
)

// SessionState represents the state of a user session.
type SessionState string

const (
	// SessionActive indicates an active session.
	SessionActive SessionState = "active"
	// SessionExpired indicates an expired session.
	SessionExpired SessionState = "expired"
	// SessionClosed indicates a manually closed session.
	SessionClosed SessionState = "closed"
)

// Session represents a user session built from tracking events.
type Session struct {
	ID            string      `json:"id"`
	UserID        string      `json:"user_id"`
	UserType      UserType    `json:"user_type"`
	DeviceType    DeviceType  `json:"device_type"`
	State         SessionState `json:"state"`
	StartTime     time.Time   `json:"start_time"`
	EndTime       time.Time   `json:"end_time"`
	LastEventTime time.Time   `json:"last_event_time"`
	EventCount    int         `json:"event_count"`
	Pages         []string    `json:"pages"`
	TotalDuration int64       `json:"total_duration_ms"`
	Referrer      string      `json:"referrer"`
	Country       string      `json:"country"`
	CreatedAt     time.Time   `json:"created_at"`
	UpdatedAt     time.Time   `json:"updated_at"`
}

// NewSession creates a new Session.
func NewSession(userID string, deviceType DeviceType, timeout time.Duration) *Session {
	now := time.Now()
	return &Session{
		ID:         generateSessionID(),
		UserID:     userID,
		UserType:   UserNew,
		DeviceType: deviceType,
		State:      SessionActive,
		StartTime:  now,
		EndTime:    now.Add(timeout),
		CreatedAt:  now,
		UpdatedAt:  now,
		Pages:      make([]string, 0),
	}
}

// IsExpired checks if the session has expired.
func (s *Session) IsExpired() bool {
	return time.Now().After(s.EndTime) || s.State == SessionExpired
}

// IsActive checks if the session is still active.
func (s *Session) IsActive() bool {
	return s.State == SessionActive && !time.Now().After(s.EndTime)
}

// AddEvent adds an event to the session and updates session metadata.
func (s *Session) AddEvent(event *Event, timeout time.Duration) {
	s.LastEventTime = event.Timestamp
	s.EventCount++
	s.EndTime = event.Timestamp.Add(timeout)
	s.UpdatedAt = time.Now()

	if event.Type == EventPageView {
		s.Pages = append(s.Pages, event.PageURL)
	}
	if event.Referrer != "" {
		s.Referrer = event.Referrer
	}
	if event.Country != "" {
		s.Country = event.Country
	}
}

// ComputeDuration calculates the total session duration.
func (s *Session) ComputeDuration() int64 {
	if s.LastEventTime.IsZero() {
		return 0
	}
	return s.LastEventTime.Sub(s.StartTime).Milliseconds()
}

// Complete marks the session as completed.
func (s *Session) Complete() {
	s.State = SessionClosed
	s.TotalDuration = s.ComputeDuration()
	s.UpdatedAt = time.Now()
}

// SessionQuery is the query parameters for listing sessions.
type SessionQuery struct {
	UserID        string       `json:"user_id"`
	State         SessionState `json:"state"`
	UserType      UserType     `json:"user_type"`
	DeviceType    DeviceType   `json:"device_type"`
	StartDate     time.Time    `json:"start_date"`
	EndDate       time.Time    `json:"end_date"`
	MinDurationMs int64        `json:"min_duration_ms"`
	Page          int          `json:"page"`
	PageSize      int          `json:"page_size"`
}

// Validate checks if session query parameters are valid.
func (q *SessionQuery) Validate() error {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize <= 0 || q.PageSize > 500 {
		q.PageSize = 50
	}
	if q.StartDate.After(q.EndDate) && !q.EndDate.IsZero() {
		return ErrInvalidTimeRange
	}
	return nil
}

// SessionStats contains session aggregation statistics.
type SessionStats struct {
	TotalSessions     int64   `json:"total_sessions"`
	ActiveSessions    int64   `json:"active_sessions"`
	ExpiredSessions   int64   `json:"expired_sessions"`
	NewUsers          int64   `json:"new_users"`
	ReturningUsers    int64   `json:"returning_users"`
	AvgDurationMs     float64 `json:"avg_duration_ms"`
	AvgPagesPerSession float64 `json:"avg_pages_per_session"`
	UniqueUsers       int64   `json:"unique_users"`
}
