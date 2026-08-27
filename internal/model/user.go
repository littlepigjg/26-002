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

// Update updates user dimension from an event.
func (ud *UserDimension) Update(event *Event) {
	ud.LastSeenAt = event.Timestamp
	ud.EventCount++

	if event.DeviceType != "" {
		ud.DeviceType = event.DeviceType
	}
	if event.OS != "" {
		ud.OS = event.OS
	}
	if event.Browser != "" {
		ud.Browser = event.Browser
	}
	if event.Country != "" {
		ud.Country = event.Country
	}
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
