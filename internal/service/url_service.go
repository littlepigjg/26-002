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
	"github.com/ubaas/ubaas/pkg/logger"
)

// URLService handles short URL creation and management.
type URLService struct {
	cfg   *config.Config
	store *store.URLStore
	logger *logger.Logger
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
		cfg:    cfg,
		store:  s,
		logger: logger.New(nil, 2, "url_service"),
	}, nil
}

// Create creates a new short URL.
func (us *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	code := req.CustomCode
	if code == "" {
		var b [4]byte
		rand.Read(b[:])
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

	overwrite := req.CustomCode != ""
	if err := us.store.Save(shortURL, overwrite); err != nil {
		return nil, err
	}

	return shortURL, nil
}

// Get retrieves a short URL by its code.
func (us *URLService) Get(ctx context.Context, code string) (*model.ShortURL, error) {
	if err := ctx.Err(); err != nil {
		return nil, err
	}
	return us.store.Get(code)
}

// Disable disables a short URL.
func (us *URLService) Disable(ctx context.Context, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	u, err := us.store.Get(code)
	if err != nil {
		return err
	}

	u.Disabled = true
	return us.store.Save(u, true)
}

// Enable enables a disabled short URL.
func (us *URLService) Enable(ctx context.Context, code string) error {
	if err := ctx.Err(); err != nil {
		return err
	}

	u, err := us.store.Get(code)
	if err != nil {
		return err
	}

	u.Disabled = false
	return us.store.Save(u, true)
}

// RedirectRequest represents a request to follow a short URL redirect.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult represents the result of a redirect operation.
type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

// RedirectService handles short URL redirect operations.
type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
	logger    *logger.Logger
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("access log store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
		logger:   logger.New(nil, 2, "redirect_service"),
	}, nil
}

// HandleRedirect handles a redirect request for a short URL.
func (rs *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req.Code == "" {
		return nil, model.ErrInvalidRequest
	}

	if err := ctx.Err(); err != nil {
		return nil, err
	}

	u, err := rs.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, nil
	}

	if u.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if u.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	rs.urlStore.IncrementVisits(req.Code)

	rs.logStore.Log(req.Code, "", "", "", 302)

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
