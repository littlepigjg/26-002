package model

import (
	"errors"
	"time"
)

var (
	ErrInvalidURL    = errors.New("invalid url")
	ErrCodeExists    = errors.New("code already exists")
	ErrURLNotFound   = errors.New("url not found")
	ErrRedirectLimit = errors.New("redirect limit exceeded")
)

// CreateReq represents a request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks if the CreateReq is valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return ErrInvalidURL
	}
	if len(r.RawURL) > 2048 {
		return ErrInvalidURL
	}
	return nil
}

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
	MaxVisits int       `json:"max_visits"`
}

// Validate checks if the ShortURL is valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return ErrInvalidURL
	}
	if s.RawURL == "" {
		return ErrInvalidURL
	}
	return nil
}

// IsExpired checks if the ShortURL has expired based on max visits.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	return false
}


