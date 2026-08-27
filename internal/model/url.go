package model

import "time"

type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return ErrInvalidRequest
	}
	if r.MaxVisits < 0 {
		return ErrInvalidRequest
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
	ExpiresAt time.Time `json:"expires_at,omitempty"`
}

func (u *ShortURL) Validate() error {
	if u.Code == "" || u.RawURL == "" {
		return ErrInvalidRequest
	}
	return nil
}

func (u *ShortURL) IsExpired(now time.Time) bool {
	if u.ExpiresAt.IsZero() {
		return false
	}
	return now.After(u.ExpiresAt)
}