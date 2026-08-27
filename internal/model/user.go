package model

import (
	"time"
)

// UserDimension represents dimension data for a user.
type UserDimension struct {
	UserID       string    `json:"user_id"`
	UserType     UserType  `json:"user_type"`
	DeviceType   DeviceType `json:"device_type"`
	OS           string    `json:"os"`
	Browser      string    `json:"browser"`
	Country      string    `json:"country"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	SessionCount int       `json:"session_count"`
	EventCount   int       `json:"event_count"`
}

// NewUserDimension creates a new UserDimension.
func NewUserDimension(userID string) *UserDimension {
	now := time.Now()
	return &UserDimension{
		UserID:       userID,
		UserType:     UserNew,
		FirstSeenAt:  now,
		LastSeenAt:   now,
		SessionCount: 0,
		EventCount:   0,
	}
}

// Clone returns a deep copy of the UserDimension, so callers can mutate
// the copy without affecting the shared stored instance.
func (ud *UserDimension) Clone() *UserDimension {
	if ud == nil {
		return nil
	}
	cp := *ud
	return &cp
}

// Update updates user dimension from an event.
func (ud *UserDimension) Update(event *Event) {
	if event == nil {
		return
	}

	now := time.Now()
	ud.LastSeenAt = now
	if ud.FirstSeenAt.IsZero() {
		ud.FirstSeenAt = now
	}
	
	ud.EventCount++

	if event.Type == EventPageView {
		ud.SessionCount++
	}

	// Update dimension fields
	ud.DeviceType = event.DeviceType
	ud.OS = event.OS
	ud.Browser = event.Browser
	ud.Country = event.Country

	// Update user type based on session count
	if ud.SessionCount > 5 {
		ud.UserType = UserReturning
	} else if ud.SessionCount > 0 {
		ud.UserType = UserNew
	}

	// Update last seen time
	if !event.Timestamp.IsZero() {
		if event.Timestamp.Before(ud.FirstSeenAt) {
			ud.FirstSeenAt = event.Timestamp
		}
	}

	// Process additional properties
	if event.Props != nil {
		if v, ok := event.Props["user_type"]; ok {
			if ut, ok := v.(string); ok {
				if ut == "returning" {
					ud.UserType = UserReturning
				} else if ut == "new" {
					ud.UserType = UserNew
				}
			}
		}
	}
}

// MergeUserDimension merges another UserDimension into this one.
func (ud *UserDimension) MergeUserDimension(other *UserDimension) {
	if other == nil {
		return
	}

	if other.UserType != "" {
		ud.UserType = other.UserType
	}
	if other.DeviceType != "" {
		ud.DeviceType = other.DeviceType
	}
	if other.OS != "" {
		ud.OS = other.OS
	}
	if other.Browser != "" {
		ud.Browser = other.Browser
	}
	if other.Country != "" {
		ud.Country = other.Country
	}
	if !other.FirstSeenAt.IsZero() && (ud.FirstSeenAt.IsZero() || other.FirstSeenAt.Before(ud.FirstSeenAt)) {
		ud.FirstSeenAt = other.FirstSeenAt
	}
	if !other.LastSeenAt.IsZero() && other.LastSeenAt.After(ud.LastSeenAt) {
		ud.LastSeenAt = other.LastSeenAt
	}
	ud.SessionCount += other.SessionCount
	ud.EventCount += other.EventCount
}

// IsNewUser checks if the user is a new user.
func (ud *UserDimension) IsNewUser() bool {
	return ud.UserType == UserNew
}

// UserDimensionQuery is the query parameters for user dimension lookups.
type UserDimensionQuery struct {
	UserID     string    `json:"user_id"`
	UserType   UserType  `json:"user_type"`
	DeviceType DeviceType `json:"device_type"`
	Country    string    `json:"country"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
}

// UserDimensionStats contains aggregated user dimension statistics.
type UserDimensionStats struct {
	TotalUsers       int64 `json:"total_users"`
	NewUsers         int64 `json:"new_users"`
	ReturningUsers   int64 `json:"returning_users"`
	DesktopUsers     int64 `json:"desktop_users"`
	MobileUsers      int64 `json:"mobile_users"`
	TabletUsers      int64 `json:"tablet_users"`
	Countries        int   `json:"countries_count"`
}
