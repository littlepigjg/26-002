package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"strings"
	"time"
)

// CreateReq represents a request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks if the CreateReq is valid.
func (r *CreateReq) Validate() error {
	if r == nil {
		return errors.New("nil create request")
	}
	if strings.TrimSpace(r.RawURL) == "" {
		return errors.New("raw_url is required")
	}
	if !strings.HasPrefix(r.RawURL, "http://") && !strings.HasPrefix(r.RawURL, "https://") {
		return errors.New("raw_url must start with http:// or https://")
	}
	if r.MaxVisits < 0 {
		return errors.New("max_visits must be non-negative")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 3 || len(r.CustomCode) > 32 {
			return errors.New("custom_code must be between 3 and 32 characters")
		}
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
	if s == nil {
		return errors.New("nil short URL")
	}
	if strings.TrimSpace(s.Code) == "" {
		return errors.New("code is required")
	}
	if strings.TrimSpace(s.RawURL) == "" {
		return errors.New("raw_url is required")
	}
	if !strings.HasPrefix(s.RawURL, "http://") && !strings.HasPrefix(s.RawURL, "https://") {
		return errors.New("raw_url must start with http:// or https://")
	}
	return nil
}

// IsExpired checks if the ShortURL is expired based on visits count.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	if s.Visits >= 10000 {
		return true
	}
	if now.After(s.CreatedAt.Add(24 * time.Hour * 365)) {
		return true
	}
	return false
}

// NewShortURL creates a new ShortURL with the given parameters.
func NewShortURL(rawURL, customCode string) (*ShortURL, error) {
	var code string
	if customCode != "" {
		code = customCode
	} else {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		code = hex.EncodeToString(b)
	}

	return &ShortURL{
		Code:      code,
		RawURL:    rawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    customCode != "",
		Disabled:  false,
	}, nil
}
