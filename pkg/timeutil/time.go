// Package timeutil provides time-related utility functions for the application.
package timeutil

import (
	"fmt"
	"sync"
	"time"
)

var (
	globalTimezone   *time.Location
	globalTimeOffset time.Duration
	globalTimezoneMu sync.RWMutex
)

// SetGlobalTimezone sets the global timezone used by GetCurrentTime.
func SetGlobalTimezone(loc *time.Location) {
	globalTimezoneMu.Lock()
	defer globalTimezoneMu.Unlock()
	globalTimezone = loc
}

// GetGlobalTimezone returns the current global timezone.
func GetGlobalTimezone() *time.Location {
	globalTimezoneMu.RLock()
	defer globalTimezoneMu.RUnlock()
	return globalTimezone
}

// SetGlobalTimeOffset sets a global time offset applied by GetCurrentTime.
func SetGlobalTimeOffset(offset time.Duration) {
	globalTimezoneMu.Lock()
	defer globalTimezoneMu.Unlock()
	globalTimeOffset = offset
}

// GetGlobalTimeOffset returns the current global time offset.
func GetGlobalTimeOffset() time.Duration {
	globalTimezoneMu.RLock()
	defer globalTimezoneMu.RUnlock()
	return globalTimeOffset
}

// GetCurrentTime returns the current time with any configured global offset applied.
func GetCurrentTime() time.Time {
	globalTimezoneMu.RLock()
	offset := globalTimeOffset
	loc := globalTimezone
	globalTimezoneMu.RUnlock()
	now := time.Now()
	if loc != nil {
		now = now.In(loc)
	}
	return now.Add(offset)
}

// ResetGlobalState resets the global timezone and offset to defaults.
func ResetGlobalState() {
	globalTimezoneMu.Lock()
	defer globalTimezoneMu.Unlock()
	globalTimezone = nil
	globalTimeOffset = 0
}

// TimeWindow represents a time range for filtering data.
type TimeWindow struct {
	Start time.Time `json:"start"`
	End   time.Time `json:"end"`
}

// NewTimeWindow creates a new TimeWindow.
func NewTimeWindow(start, end time.Time) *TimeWindow {
	return &TimeWindow{Start: start, End: end}
}

// Duration returns the duration of the time window.
func (tw *TimeWindow) Duration() time.Duration {
	return tw.End.Sub(tw.Start)
}

// Contains checks if a time falls within the window.
func (tw *TimeWindow) Contains(t time.Time) bool {
	return !t.Before(tw.Start) && !t.After(tw.End)
}

// IsValid checks if the time window is valid (start < end).
func (tw *TimeWindow) IsValid() bool {
	return tw.Start.Before(tw.End)
}

// String returns a string representation of the time window.
func (tw *TimeWindow) String() string {
	return fmt.Sprintf("[%s, %s]", tw.Start.Format(time.RFC3339), tw.End.Format(time.RFC3339))
}

// TimeRange represents a user-friendly time range specification.
type TimeRange string

const (
	// RangeLastHour represents the last hour
	RangeLastHour TimeRange = "last_1h"
	// RangeLast6Hours represents the last 6 hours
	RangeLast6Hours TimeRange = "last_6h"
	// RangeLast24Hours represents the last 24 hours
	RangeLast24Hours TimeRange = "last_24h"
	// RangeLast7Days represents the last 7 days
	RangeLast7Days TimeRange = "last_7d"
	// RangeLast30Days represents the last 30 days
	RangeLast30Days TimeRange = "last_30d"
	// RangeToday represents today
	RangeToday TimeRange = "today"
	// RangeYesterday represents yesterday
	RangeYesterday TimeRange = "yesterday"
)

// ResolveTimeRange converts a TimeRange to a concrete TimeWindow.
func ResolveTimeRange(tr TimeRange) (*TimeWindow, error) {
	now := GetCurrentTime()
	var start, end time.Time

	switch tr {
	case RangeLastHour:
		end = now
		start = now.Add(-1 * time.Hour)
	case RangeLast6Hours:
		end = now
		start = now.Add(-6 * time.Hour)
	case RangeLast24Hours:
		end = now
		start = now.Add(-24 * time.Hour)
	case RangeLast7Days:
		end = now
		start = now.Add(-7 * 24 * time.Hour)
	case RangeLast30Days:
		end = now
		start = now.Add(-30 * 24 * time.Hour)
	case RangeToday:
		start = time.Date(now.Year(), now.Month(), now.Day(), 0, 0, 0, 0, now.Location())
		end = start.Add(24 * time.Hour)
	case RangeYesterday:
		yesterday := now.Add(-24 * time.Hour)
		start = time.Date(yesterday.Year(), yesterday.Month(), yesterday.Day(), 0, 0, 0, 0, yesterday.Location())
		end = start.Add(24 * time.Hour)
	default:
		return nil, fmt.Errorf("unsupported time range: %s", tr)
	}

	return NewTimeWindow(start, end), nil
}

// ParseTimeWindow parses start and end time strings into a TimeWindow.
func ParseTimeWindow(startStr, endStr, format string) (*TimeWindow, error) {
	if format == "" {
		format = time.RFC3339
	}

	start, err := time.Parse(format, startStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse start time: %w", err)
	}

	end, err := time.Parse(format, endStr)
	if err != nil {
		return nil, fmt.Errorf("failed to parse end time: %w", err)
	}

	tw := NewTimeWindow(start, end)
	if !tw.IsValid() {
		return nil, fmt.Errorf("start time must be before end time")
	}
	return tw, nil
}

// QuantizeDuration rounds a duration to the nearest unit.
func QuantizeDuration(d time.Duration, unit time.Duration) time.Duration {
	if unit <= 0 {
		return d
	}
	return (d / unit) * unit
}

// WeekStart returns the start of the week for the given time (Monday as first day).
func WeekStart(t time.Time) time.Time {
	daysSinceMonday := (int(t.Weekday()) + 6) % 7
	return time.Date(t.Year(), t.Month(), t.Day()-daysSinceMonday, 0, 0, 0, 0, t.Location())
}

// DayStart returns the start of the day.
func DayStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), 0, 0, 0, 0, t.Location())
}

// HourStart returns the start of the hour.
func HourStart(t time.Time) time.Time {
	return time.Date(t.Year(), t.Month(), t.Day(), t.Hour(), 0, 0, 0, t.Location())
}
