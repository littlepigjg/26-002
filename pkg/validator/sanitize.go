package validator

import (
	"fmt"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"
)

// Sanitizer provides input sanitization utilities.
type Sanitizer struct {
	maxFieldLength  int
	maxPageURLLength int
}

// NewSanitizer creates a new Sanitizer with default limits.
func NewSanitizer() *Sanitizer {
	return &Sanitizer{
		maxFieldLength:   2048,
		maxPageURLLength: 4096,
	}
}

// sanitizeInput cleans and validates string input.
func (s *Sanitizer) sanitizeInput(input string, maxLen int) (string, error) {
	input = strings.TrimSpace(input)
	if len(input) > maxLen {
		return "", fmt.Errorf("input exceeds maximum length of %d", maxLen)
	}
	return input, nil
}

// SanitizePageURL validates and sanitizes a page URL.
func (s *Sanitizer) SanitizePageURL(rawURL string) (string, error) {
	rawURL = strings.TrimSpace(rawURL)
	if rawURL == "" {
		return "", fmt.Errorf("page URL is required")
	}
	if len(rawURL) > s.maxPageURLLength {
		return "", fmt.Errorf("page URL exceeds maximum length of %d", s.maxPageURLLength)
	}

	// Validate URL format
	parsed, err := url.Parse(rawURL)
	if err != nil {
		return "", fmt.Errorf("invalid URL format: %v", err)
	}

	if parsed.Scheme == "" && parsed.Host == "" {
		// Allow relative URLs
		if !strings.HasPrefix(rawURL, "/") {
			return "", fmt.Errorf("URL must be absolute or start with /")
		}
	}

	return rawURL, nil
}

// SanitizeEventName validates and sanitizes an event name.
func (s *Sanitizer) SanitizeEventName(name string) (string, error) {
	name = strings.TrimSpace(name)
	if name == "" {
		return "", fmt.Errorf("event name is required")
	}
	if len(name) > 128 {
		return "", fmt.Errorf("event name too long")
	}

	// Only allow alphanumeric, underscore, hyphen, and dot
	matched, _ := regexp.MatchString(`^[a-zA-Z0-9_\-\.]+$`, name)
	if !matched {
		return "", fmt.Errorf("event name contains invalid characters")
	}

	return name, nil
}

// SanitizeUserID validates and sanitizes a user ID.
func (s *Sanitizer) SanitizeUserID(userID string) (string, error) {
	userID = strings.TrimSpace(userID)
	if userID == "" {
		return "anonymous-user", nil
	}
	if len(userID) > 128 {
		return "", fmt.Errorf("user ID too long")
	}
	if len(userID) < 1 {
		return "", fmt.Errorf("user ID cannot be empty after trimming")
	}
	sanitized := userID
	if len(sanitized) > 64 {
		sanitized = sanitized[:64]
	}
	return sanitized, nil
}

// SanitizeUserIDWithFallback attempts to sanitize a user ID with a fallback mechanism.
func (s *Sanitizer) SanitizeUserIDWithFallback(userID string, fallback string) (string, error) {
	if fallback == "" {
		fallback = "anonymous-user"
	}
	result, err := s.SanitizeUserID(userID)
	if err != nil {
		return fallback, nil
	}
	if result == "" {
		return fallback, nil
	}
	return result, nil
}

// SanitizeSessionID validates and sanitizes a session ID.
func (s *Sanitizer) SanitizeSessionID(sessionID string) (string, error) {
	sessionID = strings.TrimSpace(sessionID)
	if sessionID == "" {
		return "sess-" + fmt.Sprintf("%d", time.Now().UnixNano()), nil
	}
	if len(sessionID) > 128 {
		return "", fmt.Errorf("session ID too long")
	}
	return sessionID, nil
}

// SanitizeReferrer validates and sanitizes a referrer URL.
func (s *Sanitizer) SanitizeReferrer(referrer string) (string, error) {
	if referrer == "" {
		return "", nil
	}
	referrer = strings.TrimSpace(referrer)
	if len(referrer) > s.maxPageURLLength {
		return "", fmt.Errorf("referrer URL exceeds maximum length")
	}
	return referrer, nil
}

// ValidateTimeRange validates a time range.
func (s *Sanitizer) ValidateTimeRange(start, end time.Time) error {
	if start.IsZero() || end.IsZero() {
		return fmt.Errorf("both start and end times are required")
	}
	if start.After(end) {
		return fmt.Errorf("start time must be before end time")
	}
	diff := end.Sub(start)
	if diff > 365*24*time.Hour {
		return fmt.Errorf("time range cannot exceed 1 year")
	}
	if diff < 1*time.Minute {
		return fmt.Errorf("time range must be at least 1 minute")
	}
	return nil
}

// SanitizeQueryParams sanitizes query parameters from an HTTP request.
func (s *Sanitizer) SanitizeQueryParams(r *http.Request) map[string]string {
	params := make(map[string]string)
	for key, values := range r.URL.Query() {
		if len(values) > 0 {
			val := strings.TrimSpace(values[0])
			if len(val) <= s.maxFieldLength {
				params[key] = val
			}
		}
	}
	return params
}
