package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
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
		cfg = config.Default()
	}
	return &URLService{cfg: cfg, store: s}, nil
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}
	code := req.CustomCode
	if code == "" {
		var b [6]byte
		_, _ = rand.Read(b[:])
		code = hex.EncodeToString(b[:])
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

type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

type RedirectService struct {
	urlStore *store.URLStore
	logStore *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	return &RedirectService{urlStore: us, logStore: ls}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}
	if u.Disabled {
		return &RedirectResult{RawURL: "", Status: 410}, nil
	}
	if u.IsExpired(req.Timestamp) {
		return &RedirectResult{RawURL: "", Status: 410}, nil
	}
	s.logStore.Append(req.Timestamp)
	return &RedirectResult{RawURL: u.RawURL, Status: 302}, nil
}