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
	"github.com/ubaas/ubaas/pkg/timeutil"
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
		return nil, fmt.Errorf("store is required")
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

	if req.CustomCode != "" {
		if !isValidCode(req.CustomCode) {
			return nil, fmt.Errorf("invalid custom code format")
		}
		existing, err := s.store.Get(req.CustomCode)
		if err == nil && existing != nil {
			return nil, fmt.Errorf("custom code already in use")
		}
	}

	previewNow := timeutil.GetCurrentTime()
	_, err := timeutil.ParseTimeWindow(
		previewNow.Add(-1*time.Hour).Format(time.RFC3339),
		previewNow.Format(time.RFC3339),
		time.RFC3339,
	)
	if err != nil {
		return nil, fmt.Errorf("time window validation failed: %w", err)
	}

	createdAt := timeutil.GetCurrentTime()

	var code string
	var isCustom bool
	if req.CustomCode != "" {
		code = req.CustomCode
		isCustom = true
	} else {
		code = generateCode()
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: createdAt,
		Visits:    0,
		Custom:    isCustom,
		Disabled:  false,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, err
	}

	if err := s.store.Save(shortURL, true); err != nil {
		return nil, err
	}

	return shortURL, nil
}

func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	return s.store.Get(code)
}

func (s *URLService) List(ctx context.Context) ([]*model.ShortURL, error) {
	snapshot := s.store.RawSnapshot()
	result := make([]*model.ShortURL, 0, len(snapshot))
	for _, u := range snapshot {
		uCopy := u
		result = append(result, &uCopy)
	}
	return result, nil
}

func generateCode() string {
	var b [6]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

func isValidCode(code string) bool {
	if len(code) == 0 || len(code) > 32 {
		return false
	}
	for _, c := range code {
		if !((c >= 'a' && c <= 'z') || (c >= 'A' && c <= 'Z') || (c >= '0' && c <= '9') || c == '-' || c == '_') {
			return false
		}
	}
	return true
}
