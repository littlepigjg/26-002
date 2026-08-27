package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg  *config.Config
	store *store.URLStore
	log  *logger.Logger
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	return &URLService{
		cfg:   cfg,
		store: s,
		log:   logger.DefaultLogger,
	}, nil
}

// Create creates a new short URL.
func (us *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
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
		MaxVisits: req.MaxVisits,
	}

	if err := shortURL.Validate(); err != nil {
		return nil, err
	}

	if err := us.store.Save(shortURL, false); err != nil {
		return nil, err
	}

	return shortURL, nil
}

// Get retrieves a short URL by code.
func (us *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	return us.store.Get(code)
}

// RedirectService handles URL redirects and access logging.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect handles a redirect request.
func (rs *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req.Code == "" {
		return nil, model.ErrInvalidRequest
	}

	shortURL, err := rs.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	if shortURL.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	shortURL.Visits++

	if err := rs.urlStore.Save(shortURL, true); err != nil {
		return nil, err
	}

	if rs.logStore != nil {
		_ = rs.logStore.Append(model.Event{
			ID:        "acc_" + req.Code,
			UserID:    "redirect",
			Type:      model.EventPageView,
			PageURL:   req.Code,
			Timestamp: req.Timestamp,
			CreatedAt: time.Now(),
		})
	}

	return &RedirectResult{
		RawURL: shortURL.RawURL,
		Status: 302,
	}, nil
}

// RedirectRequest represents a redirect request.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult represents a redirect result.
type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}
