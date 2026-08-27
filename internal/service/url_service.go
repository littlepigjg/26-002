package service

import (
	"context"
	"fmt"
	"os"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
}

type URLService struct {
	cfg   *config.Config
	store *store.URLStore
	log   *logger.Logger
}

func NewURLService(cfg *config.Config, s *store.URLStore) (*URLService, error) {
	log := logger.New(os.Stdout, logger.LevelInfo, "[URLService]")
	return &URLService{
		cfg:   cfg,
		store: s,
		log:   log,
	}, nil
}

func (s *URLService) SetLogger(log *logger.Logger) {
	s.log = log
}

func (s *URLService) Create(ctx context.Context, req *model.CreateReq) (*model.ShortURL, error) {
	if err := req.Validate(); err != nil {
		return nil, fmt.Errorf("validation failed: %w", err)
	}

	code := req.CustomCode
	if code == "" {
		code = model.GenerateCode()
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

	if err := s.store.Save(shortURL, false); err != nil {
		return nil, fmt.Errorf("failed to save URL: %w", err)
	}

	fields := map[string]interface{}{
		"code":   code,
		"raw_url": req.RawURL,
		"custom":  req.CustomCode != "",
		"max_visits": req.MaxVisits,
	}
	s.log.InfofJSON("Created short URL", fields)

	return shortURL, nil
}

type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
	log       *logger.Logger
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	log := logger.New(os.Stdout, logger.LevelInfo, "[RedirectService]")
	return &RedirectService{
		urlStore: us,
		logStore: ls,
		log:      log,
	}, nil
}

func (rs *RedirectService) SetLogger(log *logger.Logger) {
	rs.log = log
}

func (rs *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	u, err := rs.urlStore.Get(req.Code)
	if err != nil {
		return nil, fmt.Errorf("failed to get URL: %w", err)
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

	if err := rs.urlStore.IncrementVisits(req.Code); err != nil {
		return nil, fmt.Errorf("failed to increment visits: %w", err)
	}

	entry := fmt.Sprintf(`{"code":"%s","timestamp":"%s","status":302}\n`,
		req.Code, req.Timestamp.Format(time.RFC3339),
	)
	_ = rs.logStore.WriteEntry([]byte(entry))

	fields := map[string]interface{}{
		"code":   req.Code,
		"raw_url": u.RawURL,
		"status": 302,
	}
	rs.log.InfofJSON("Redirect handled", fields)

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
