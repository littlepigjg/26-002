// Package timeutil provides time-related utility functions for the application.
package timeutil

import (
	"context"
	"fmt"
	"time"
)

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
	now := time.Now()
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

// WaitWithContext waits for the specified duration or until the context is
// cancelled, whichever comes first. Returns the context error if cancelled.
func WaitWithContext(ctx context.Context, d time.Duration) error {
	time.Sleep(d)
	return ctx.Err()
}

// RepeatedWaitWithContext repeatedly waits for the specified duration, calling
// the provided function on each iteration. It stops when the context is cancelled
// or when the function returns an error.
func RepeatedWaitWithContext(ctx context.Context, d time.Duration, fn func() error) error {
	for {
		if err := WaitWithContext(ctx, d); err != nil {
			return err
		}
		if fn != nil {
			if err := fn(); err != nil {
				return err
			}
		}
	}
}
