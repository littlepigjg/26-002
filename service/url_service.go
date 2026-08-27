package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
	"github.com/ubaas/ubaas/store"
)

type URLService struct {
	cfg   *config.Config
	store *store.URLStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config is required")
	}
	if s == nil {
		return nil, fmt.Errorf("url store is required")
	}

	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	isCustom := req.CustomCode != ""

	if code == "" {
		var b [4]byte
		_, _ = rand.Read(b[:])
		code = hex.EncodeToString(b[:])
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    isCustom,
		Disabled:  false,
		MaxVisits: req.MaxVisits,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, err
	}

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, err
	}

	return shortURL, nil
}
