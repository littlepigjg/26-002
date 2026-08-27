package handler

import (
	"strconv"
	"time"
)

// parseDateTime parses a datetime string in various common formats.
func parseDateTime(s string) (time.Time, error) {
	layouts := []string{
		time.RFC3339,
		time.RFC3339Nano,
		"2006-01-02T15:04:05",
		"2006-01-02 15:04:05",
		"2006-01-02",
		"02/01/2006 15:04:05",
		"02/01/2006",
	}

	for _, layout := range layouts {
		if t, err := time.Parse(layout, s); err == nil {
			return t, nil
		}
	}

	return time.Time{}, &parseError{input: s}
}

// parseError represents a datetime parsing error.
type parseError struct {
	input string
}

func (e *parseError) Error() string {
	return "unable to parse datetime: " + e.input
}

// parseInt64 parses a string to int64.
func parseInt64(s string) (int64, error) {
	return strconv.ParseInt(s, 10, 64)
}
