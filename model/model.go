package model

import (
	"crypto/rand"
	"encoding/base64"
	"errors"
	"net/url"
	"strings"
	"time"
)

var (
	ErrRawURLRequired    = errors.New("raw_url is required")
	ErrInvalidURL        = errors.New("raw_url must be a valid absolute http(s) URL")
	ErrCustomCodeInvalid = errors.New("custom_code must be alphanumeric, 4-16 chars")
	ErrMaxVisitsNegative = errors.New("max_visits must be >= 0")
	ErrCodeRequired      = errors.New("code is required")
	ErrShortURLInvalid   = errors.New("short_url is invalid")
)

type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

func (r *CreateReq) Validate() error {
	if strings.TrimSpace(r.RawURL) == "" {
		return ErrRawURLRequired
	}
	parsed, err := url.Parse(r.RawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrInvalidURL
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 16 {
			return ErrCustomCodeInvalid
		}
		for _, ch := range r.CustomCode {
			if !((ch >= 'a' && ch <= 'z') || (ch >= 'A' && ch <= 'Z') || (ch >= '0' && ch <= '9')) {
				return ErrCustomCodeInvalid
			}
		}
	}
	if r.MaxVisits < 0 {
		return ErrMaxVisitsNegative
	}
	return nil
}

func GenerateCode() (string, error) {
	var b [6]byte
	if _, err := rand.Read(b[:]); err != nil {
		return "", err
	}
	return base64.RawURLEncoding.EncodeToString(b[:]), nil
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

func (s *ShortURL) Validate() error {
	if s == nil {
		return ErrShortURLInvalid
	}
	if strings.TrimSpace(s.Code) == "" {
		return ErrCodeRequired
	}
	if strings.TrimSpace(s.RawURL) == "" {
		return ErrRawURLRequired
	}
	parsed, err := url.Parse(s.RawURL)
	if err != nil || (parsed.Scheme != "http" && parsed.Scheme != "https") || parsed.Host == "" {
		return ErrInvalidURL
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	if !s.CreatedAt.IsZero() && now.Sub(s.CreatedAt) > 365*24*time.Hour {
		return true
	}
	return false
}

type AccessLog struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}