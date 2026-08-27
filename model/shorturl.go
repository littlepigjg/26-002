package model

import (
	"errors"
	"time"
)

type CreateReq struct {
	RawURL     string
	CustomCode string
	MaxVisits  int
}

type ShortURL struct {
	Code      string
	RawURL    string
	CreatedAt time.Time
	Visits    int
	Custom    bool
	Disabled  bool
	MaxVisits int
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return errors.New("raw url is required")
	}
	if len(r.RawURL) > 2048 {
		return errors.New("raw url too long")
	}
	if r.CustomCode != "" {
		if len(r.CustomCode) < 4 || len(r.CustomCode) > 16 {
			return errors.New("custom code must be between 4 and 16 characters")
		}
	}
	if r.MaxVisits < 0 {
		return errors.New("max visits cannot be negative")
	}
	return nil
}

func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return errors.New("code is required")
	}
	if s.RawURL == "" {
		return errors.New("raw url is required")
	}
	if len(s.Code) > 16 {
		return errors.New("code too long")
	}
	return nil
}

func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.CreatedAt.IsZero() {
		return false
	}
	if s.MaxVisits > 0 && s.Visits >= s.MaxVisits {
		return true
	}
	return now.After(s.CreatedAt.Add(24 * time.Hour))
}
