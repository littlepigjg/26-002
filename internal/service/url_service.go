package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

type URLService struct {
	cfg       *config.Config
	store     *store.URLStore
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, fmt.Errorf("config cannot be nil")
	}
	if s == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	return &URLService{
		cfg:   cfg,
		store: s,
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, model.ErrInvalidURL
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		code = model.GenerateCode()
	}

	u := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := s.store.Save(u, false); err != nil {
		return nil, err
	}

	return u, nil
}

func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	return s.store.Get(code)
}
