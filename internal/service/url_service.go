package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil || s == nil {
		return nil, fmt.Errorf("config and store cannot be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

// Create creates a new short URL.
func (u *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		code = generateCode()
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := u.store.Save(shortURL, false); err != nil {
		return nil, err
	}

	shortURL.Visits = req.MaxVisits

	return shortURL, nil
}

// Get retrieves a short URL by code.
func (u *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	return u.store.Get(code)
}

// SetPanicGuard sets the panic guard on the underlying store.
func (u *URLService) SetPanicGuard(fn store.PanicGuardFn) {
	u.store.SetPanicGuard(fn)
}

// RawSnapshot returns a snapshot of all stored URLs.
func (u *URLService) RawSnapshot() map[string]model.ShortURL {
	return u.store.RawSnapshot()
}

// generateCode generates a random short code.
func generateCode() string {
	var b [4]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}
