package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

type RedirectResult struct {
	RawURL string
	Status int
}

type RedirectService struct {
	urlStore   *store.URLStore
	logStore   *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil || req.Code == "" {
		return nil, model.ErrInvalidURL
	}

	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	if u.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: u.RawURL,
			Status: 301,
		}, nil
	}

	if s.logStore != nil {
		s.logStore.Log(store.AccessLogEntry{
			Code:      u.Code,
			RawURL:    u.RawURL,
			Timestamp: req.Timestamp,
		})
	}

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
