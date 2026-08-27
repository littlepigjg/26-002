// Package model defines the data structures used throughout the application.
// These models represent events, sessions, paths, conversions, dimensions, and statistics.
package model

import (
	"time"
)

// EventType represents the type of tracking event.
type EventType string

const (
	// EventPageView is a page view event.
	EventPageView EventType = "page_view"
	// EventClick is a click event.
	EventClick EventType = "click"
	// EventDuration is a duration/Stay event.
	EventDuration EventType = "duration"
	// EventConversion is a conversion event.
	EventConversion EventType = "conversion"
	// EventCustom is a custom event.
	EventCustom EventType = "custom"
)

// Valid checks if the EventType is valid.
func (et EventType) Valid() bool {
	switch et {
	case EventPageView, EventClick, EventDuration, EventConversion, EventCustom:
		return true
	default:
		return false
	}
}

// Event is the core tracking event model.
type Event struct {
	ID            string                 `json:"id"`
	UserID        string                 `json:"user_id"`
	SessionID     string                 `json:"session_id"`
	Type          EventType              `json:"type"`
	PageURL       string                 `json:"page_url"`
	PageTitle     string                 `json:"page_title"`
	DurationMs    int64                  `json:"duration_ms"`
	Referrer      string                 `json:"referrer"`
	DeviceType    DeviceType             `json:"device_type"`
	OS            string                 `json:"os"`
	Browser       string                 `json:"browser"`
	Country       string                 `json:"country"`
	Props         map[string]interface{} `json:"props"`
	Timestamp     time.Time              `json:"timestamp"`
	CreatedAt     time.Time              `json:"created_at"`
}

// NewEvent creates a new Event with default values.
func NewEvent(userID, sessionID string, eventType EventType, pageURL string) *Event {
	return &Event{
		ID:        generateEventID(),
		UserID:    userID,
		SessionID: sessionID,
		Type:      eventType,
		PageURL:   pageURL,
		Timestamp: time.Now(),
		CreatedAt: time.Now(),
		DeviceType: DeviceDesktop,
		Props:      make(map[string]interface{}),
	}
}

// EventCreateRequest is the request body for creating events.
type EventCreateRequest struct {
	UserID     string                 `json:"user_id"`
	SessionID  string                 `json:"session_id"`
	Type       EventType              `json:"type"`
	PageURL    string                 `json:"page_url"`
	PageTitle  string                 `json:"page_title"`
	DurationMs int64                  `json:"duration_ms"`
	Referrer   string                 `json:"referrer"`
	DeviceType DeviceType             `json:"device_type"`
	OS         string                 `json:"os"`
	Browser    string                 `json:"browser"`
	Country    string                 `json:"country"`
	Props      map[string]interface{} `json:"props"`
	Timestamp  time.Time              `json:"timestamp"`
}

// ToEvent converts a request to an Event model.
func (r *EventCreateRequest) ToEvent() *Event {
	ts := r.Timestamp
	if ts.IsZero() {
		ts = time.Now()
	}
	return &Event{
		ID:         generateEventID(),
		UserID:     r.UserID,
		SessionID:  r.SessionID,
		Type:       r.Type,
		PageURL:    r.PageURL,
		PageTitle:  r.PageTitle,
		DurationMs: r.DurationMs,
		Referrer:   r.Referrer,
		DeviceType: r.DeviceType,
		OS:         r.OS,
		Browser:    r.Browser,
		Country:    r.Country,
		Props:      r.Props,
		Timestamp:  ts,
		CreatedAt:  time.Now(),
	}
}

// EventQuery is the query parameters for listing events.
type EventQuery struct {
	UserID     string    `json:"user_id"`
	SessionID  string    `json:"session_id"`
	Type       EventType `json:"type"`
	DeviceType DeviceType `json:"device_type"`
	OS         string    `json:"os"`
	Browser    string    `json:"browser"`
	Country    string    `json:"country"`
	PageURL    string    `json:"page_url"`
	Referrer   string    `json:"referrer"`
	StartDate  time.Time `json:"start_date"`
	EndDate    time.Time `json:"end_date"`
	Page       int       `json:"page"`
	PageSize   int       `json:"page_size"`
	SortBy     string    `json:"sort_by"`
	SortOrder  string    `json:"sort_order"`
}

const (
	FullScanPageSize = -1
	DefaultPageSize  = 50
	MaxPageSize      = 1000
)

// Validate checks if query parameters are valid.
func (q *EventQuery) Validate() error {
	if q.Page < 1 {
		q.Page = 1
	}
	if q.PageSize == 0 {
		q.PageSize = FullScanPageSize
	} else if q.PageSize < 0 || q.PageSize > MaxPageSize {
		q.PageSize = DefaultPageSize
	}
	if q.StartDate.After(q.EndDate) && !q.EndDate.IsZero() {
		return ErrInvalidTimeRange
	}
	return nil
}

// IsFullScan returns true if this query requests an unrestricted result set.
func (q *EventQuery) IsFullScan() bool {
	return q.PageSize == FullScanPageSize
}

// EffectivePageSize returns the page size to use for this query.
func (q *EventQuery) EffectivePageSize() int {
	if q.IsFullScan() {
		return MaxPageSize * 100
	}
	if q.PageSize <= 0 {
		return DefaultPageSize
	}
	return q.PageSize
}

// EventStats contains aggregated event statistics.
type EventStats struct {
	EventType   EventType `json:"event_type"`
	Count       int64     `json:"count"`
	UniqueUsers int64     `json:"unique_users"`
	UniquePages int64     `json:"unique_pages"`
	AvgDuration float64   `json:"avg_duration_ms"`
}
