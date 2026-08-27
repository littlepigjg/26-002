package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"net/url"
	"time"
)

var (
	ErrInvalidURL     = errors.New("invalid URL format")
	ErrCodeExists     = errors.New("short code already exists")
	ErrCodeNotFound   = errors.New("short code not found")
	ErrDisabledURL    = errors.New("this short URL has been disabled")
	ErrMaxVisitsReached = errors.New("maximum visit limit reached")
)

// CreateReq is the request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks if the create request is valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return ErrInvalidURL
	}
	if _, err := url.ParseRequestURI(r.RawURL); err != nil {
		return ErrInvalidURL
	}
	if r.MaxVisits < 0 {
		return ErrInvalidRequest
	}
	return nil
}

// ShortURL represents a shortened URL.
type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

// Validate checks if the ShortURL is valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return ErrInvalidRequest
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
	return false
}

// generateShortCode generates a random short code for URLs.
func generateShortCode() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
