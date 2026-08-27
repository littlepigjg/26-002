package model

import (
	"errors"
	"net/url"
	"regexp"
	"time"
)

var (
	ErrInvalidURL      = errors.New("invalid URL")
	ErrInvalidCode     = errors.New("invalid short code")
	ErrURLNotFound     = errors.New("url not found")
	ErrURLDisabled     = errors.New("url is disabled")
	ErrURLExpired      = errors.New("url has expired")
	ErrMaxVisitsReached = errors.New("maximum visits reached")
)

var validCodePattern = regexp.MustCompile(`^[a-zA-Z0-9_-]{4,32}$`)

type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return ErrInvalidURL
	}
	parsed, err := url.ParseRequestURI(r.RawURL)
	if err != nil {
		return ErrInvalidURL
	}
	if parsed.Scheme != "http" && parsed.Scheme != "https" {
		return ErrInvalidURL
	}
	if parsed.Host == "" {
		return ErrInvalidURL
	}
	if r.CustomCode != "" && !validCodePattern.MatchString(r.CustomCode) {
		return ErrInvalidCode
	}
	if r.MaxVisits < 0 {
		r.MaxVisits = 0
	}
	return nil
}

type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return ErrInvalidCode
	}
	if !validCodePattern.MatchString(s.Code) {
		return ErrInvalidCode
	}
	if s.RawURL == "" {
		return ErrInvalidURL
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if s.Visits > 0 && s.Visits >= s.MaxVisits() {
		return true
	}
	return false
}

func (s *ShortURL) MaxVisits() int {
	return 0
}

func (s *ShortURL) FullURL(base string) string {
	if base == "" {
		return s.Code
	}
	return base + "/" + s.Code
}
