package service

import (
	"context"
	"fmt"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

// URLService handles short URL operations.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

// Create creates a new short URL.
func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	shortURL, err := model.NewShortURL(req.RawURL, req.CustomCode)
	if err != nil {
		return nil, err
	}

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, err
	}

	return shortURL, nil
}

// Get retrieves a short URL by code.
func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	return s.store.Get(code)
}

// List returns all short URLs.
func (s *URLService) List(ctx context.Context) map[string]model.ShortURL {
	return s.store.RawSnapshot()
}
