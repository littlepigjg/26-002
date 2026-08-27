package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"time"
)

var (
	ErrCodeExists       = errors.New("short code already exists")
	ErrInvalidURL       = errors.New("invalid URL")
	ErrInvalidCode      = errors.New("invalid short code")
	ErrURLNotFound      = errors.New("short URL not found")
	ErrURLDisabled      = errors.New("short URL is disabled")
	ErrShortLinkExpired = errors.New("short link has expired")
	ErrMaxVisitsExceeded = errors.New("max visits exceeded")
	ErrStorageNotOpen   = errors.New("storage is not open")
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
	if len(r.RawURL) > 2048 {
		return fmt.Errorf("URL too long: %d > 2048", len(r.RawURL))
	}
	if r.CustomCode != "" && len(r.CustomCode) > 32 {
		return ErrInvalidCode
	}
	if r.MaxVisits < 0 {
		return fmt.Errorf("max visits must be non-negative, got %d", r.MaxVisits)
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
	MaxVisits int       `json:"max_visits"`
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
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	if now.Sub(s.CreatedAt) > 30*24*time.Hour {
		return true
	}
	return false
}

func GenerateCode() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
