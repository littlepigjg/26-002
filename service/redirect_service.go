package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/store"
)

type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

type RedirectResult struct {
	RawURL  string
	Status  int
}

type RedirectService struct {
	urlStore  *store.URLStore
	logStore  *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("url store is required")
	}
	if ls == nil {
		return nil, fmt.Errorf("access log store is required")
	}

	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is required")
	}

	shortURL, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{Status: 404}, nil
	}

	if shortURL.Disabled {
		return &RedirectResult{Status: 410}, nil
	}

	now := time.Now()
	if shortURL.IsExpired(now) {
		return &RedirectResult{Status: 410}, nil
	}

	shortURL.Visits++
	if err := s.urlStore.Save(shortURL, true); err != nil {
		return nil, err
	}

	return &RedirectResult{
		RawURL: shortURL.RawURL,
		Status: 302,
	}, nil
}
