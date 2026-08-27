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

type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url store is nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, fmt.Errorf("failed to generate code: %v", err)
		}
		code = hex.EncodeToString(b)
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, err
	}

	overwrite := req.CustomCode != ""
	if err := s.store.Save(shortURL, overwrite); err != nil {
		return nil, err
	}

	return shortURL, nil
}
