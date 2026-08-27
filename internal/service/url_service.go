package service

import (
	"context"
	"crypto/rand"
	"encoding/hex"
	"errors"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

// URLService handles short URL creation and retrieval.
type URLService struct {
	cfg *config.Config
	s   *store.URLStore
}

// NewURLService creates a new URLService.
func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	if cfg == nil {
		return nil, errors.New("config is required")
	}
	if s == nil {
		return nil, errors.New("url store is required")
	}
	return &URLService{
		cfg: cfg,
		s:   s,
	}, nil
}

// Create creates a new short URL.
func (svc *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if req == nil {
		return nil, errors.New("request is nil")
	}
	if err := req.Validate(); err != nil {
		return nil, err
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	var code string
	if req.CustomCode != "" {
		code = req.CustomCode
	} else {
		b := make([]byte, 4)
		if _, err := rand.Read(b); err != nil {
			return nil, err
		}
		code = hex.EncodeToString(b)[:8]
	}

	shortURL := &model.ShortURL{
		Code:      code,
		RawURL:    req.RawURL,
		CreatedAt: time.Now(),
		Visits:    0,
		Custom:    req.CustomCode != "",
		Disabled:  false,
	}

	if err := svc.s.Save(shortURL, false); err != nil {
		return nil, err
	}

	return shortURL, nil
}

// Get retrieves a short URL by code.
func (svc *URLService) Get(code string) (*model.ShortURL, error) {
	return svc.s.Get(code)
}

// RedirectRequest represents a redirect request.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult represents the result of a redirect.
type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

// RedirectService handles URL redirects and logging.
type RedirectService struct {
	us *store.URLStore
	ls *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, errors.New("url store is required")
	}
	if ls == nil {
		return nil, errors.New("access log store is required")
	}
	return &RedirectService{
		us: us,
		ls: ls,
	}, nil
}

// HandleRedirect handles a redirect request.
func (rs *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, errors.New("redirect request is nil")
	}

	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	u, err := rs.us.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, err
	}

	if u.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, errors.New("short url is disabled")
	}

	if err := rs.us.IncrVisits(req.Code); err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 500,
		}, err
	}

	rs.ls.Append(store.AccessLogEntry{
		Code:      req.Code,
		Timestamp: req.Timestamp,
	})

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
