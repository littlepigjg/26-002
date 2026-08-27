package model

import (
	"errors"
	"regexp"
	"strings"
	"time"
)

var (
	ErrEmptyRawURL      = errors.New("raw URL must not be empty")
	ErrInvalidURLFormat = errors.New("raw URL has invalid format")
	ErrCodeTooLong      = errors.New("custom code exceeds maximum length")
	ErrInvalidMaxVisits = errors.New("max visits must be non-negative")
	ErrURLNotFound      = errors.New("short URL not found")
	ErrCodeExists       = errors.New("short code already exists")
	ErrURLDisabled      = errors.New("short URL is disabled")
	ErrInvalidCode      = errors.New("invalid short code")
)

var codeRegex = regexp.MustCompile(`^[a-zA-Z0-9_\-/ ]{2,32}$`)

const (
	DefaultMaxVisits = 0
	MaxCodeLength    = 32
	MinCodeLength    = 2
)

type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
	MaxVisits int       `json:"max_visits"`
}

func (r *CreateReq) Validate() error {
	if strings.TrimSpace(r.RawURL) == "" {
		return ErrEmptyRawURL
	}
	if len(r.RawURL) > 8192 {
		return ErrInvalidURLFormat
	}
	if r.MaxVisits < 0 {
		return ErrInvalidMaxVisits
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < MinCodeLength || len(r.CustomCode) > MaxCodeLength {
			return ErrCodeTooLong
		}
		if !codeRegex.MatchString(r.CustomCode) {
			return ErrInvalidCode
		}
	}
	return nil
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return ErrInvalidCode
	}
	if s.RawURL == "" {
		return ErrEmptyRawURL
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.MaxVisits == 0 {
		return false
	}
	if s.Visits >= s.MaxVisits {
		return true
	}
	return false
}
