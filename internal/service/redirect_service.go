package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/store"
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
		return nil, fmt.Errorf("url store is nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log store is nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if ctx.Err() != nil {
		return nil, ctx.Err()
	}

	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	shortURL, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, nil
	}

	if shortURL.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	if shortURL.MaxVisits > 0 && shortURL.Visits >= shortURL.MaxVisits {
		return &RedirectResult{
			RawURL: "",
			Status: 429,
		}, nil
	}

	entry := store.AccessLogEntry{
		Code:      req.Code,
		Timestamp: req.Timestamp,
	}
	_ = s.logStore.WriteEntry(entry)

	return &RedirectResult{
		RawURL: shortURL.RawURL,
		Status: 302,
	}, nil
}
