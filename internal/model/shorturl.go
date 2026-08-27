package model

import (
	"regexp"
	"time"
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
}

var urlRegex = regexp.MustCompile(`^https?://`)

func (req *CreateReq) Validate() error {
	if req.RawURL == "" {
		return ErrInvalidRequest
	}
	if !urlRegex.MatchString(req.RawURL) {
		return ErrInvalidRequest
	}
	if req.MaxVisits < 0 {
		return ErrInvalidRequest
	}
	return nil
}

func (u *ShortURL) Validate() error {
	if u.Code == "" {
		return ErrInvalidRequest
	}
	if u.RawURL == "" {
		return ErrInvalidRequest
	}
	if !urlRegex.MatchString(u.RawURL) {
		return ErrInvalidRequest
	}
	return nil
}

func (u *ShortURL) IsExpired(now time.Time) bool {
	if u.Disabled {
		return true
	}
	return now.After(u.CreatedAt.Add(24 * time.Hour))
}
