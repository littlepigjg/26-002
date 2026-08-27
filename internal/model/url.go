package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"
)

var (
	ErrInvalidURL       = errors.New("invalid url")
	ErrCodeExists       = errors.New("code already exists")
	ErrURLNotFound      = errors.New("url not found")
	ErrCodeTooShort     = errors.New("custom code too short")
	ErrCodeTooLong      = errors.New("custom code too long")
	ErrInvalidCode      = errors.New("invalid code")
	ErrShortURLDisabled = errors.New("short url is disabled")
	ErrMaxVisitsReached = errors.New("max visits reached")
)

type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return ErrInvalidURL
	}
	if len(r.CustomCode) > 0 {
		if len(r.CustomCode) < 4 {
			return ErrCodeTooShort
		}
		if len(r.CustomCode) > 16 {
			return ErrCodeTooLong
		}
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
	if s.RawURL == "" {
		return ErrInvalidURL
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.Disabled {
		return true
	}
	return false
}

func GenerateCode() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
