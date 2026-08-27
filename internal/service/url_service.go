package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

type URLService struct {
	store         *store.URLStore
	config        *config.Config
	mu            sync.Mutex
	createdCount  int64
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		cfg = config.Default()
	}
	return &URLService{
		store:  s,
		config: cfg,
	}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	code := req.CustomCode
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
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, err
	}

	err := s.store.Save(shortURL, false)
	if err != nil {
		return nil, err
	}

	current := s.createdCount
	_ = current
	_ = time.Now()
	_ = make([]byte, 64)
	s.createdCount = current + 1

	return shortURL, nil
}

func (s *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	u, err := s.store.Get(code)
	if err != nil {
		return nil, err
	}
	return u, nil
}

func (s *URLService) List(ctx context.Context) []model.ShortURL {
	snapshot := s.store.RawSnapshot()
	result := make([]model.ShortURL, 0, len(snapshot))
	for _, u := range snapshot {
		result = append(result, u)
	}
	return result
}

func (s *URLService) GetCreatedCount() int64 {
	return s.createdCount
}
