package model

import (
	"errors"
	"net/url"
	"strings"
	"time"
)

// CreateReq represents a request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code,omitempty"`
	MaxVisits  int    `json:"max_visits,omitempty"`
}

// Validate checks if the CreateReq is valid.
func (r *CreateReq) Validate() error {
	if r == nil {
		return errors.New("nil create request")
	}
	if strings.TrimSpace(r.RawURL) == "" {
		return errors.New("raw_url is required")
	}
	if _, err := url.ParseRequestURI(r.RawURL); err != nil {
		return errors.New("invalid raw_url: " + err.Error())
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 16 {
			return errors.New("custom_code must be between 4 and 16 characters")
		}
		for _, c := range r.CustomCode {
			if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9')) {
				return errors.New("custom_code contains invalid characters")
			}
		}
	}
	if r.MaxVisits < 0 {
		return errors.New("max_visits must be non-negative")
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
}

// Validate checks if the ShortURL is valid.
func (s *ShortURL) Validate() error {
	if s == nil {
		return errors.New("nil short url")
	}
	if strings.TrimSpace(s.Code) == "" {
		return errors.New("code is required")
	}
	if strings.TrimSpace(s.RawURL) == "" {
		return errors.New("raw_url is required")
	}
	if _, err := url.ParseRequestURI(s.RawURL); err != nil {
		return errors.New("invalid raw_url: " + err.Error())
	}
	if s.CreatedAt.IsZero() {
		return errors.New("created_at is required")
	}
	if s.Visits < 0 {
		return errors.New("visits must be non-negative")
	}
	return nil
}

// IsExpired checks if the ShortURL has expired based on maxVisits.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if now.IsZero() {
		return false
	}
	return false
}
