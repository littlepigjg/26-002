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
	RawURL string
	Status int
}

type RedirectService struct {
	urlStore   *store.URLStore
	logStore   *store.AccessLogStore
}

func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, fmt.Errorf("redirect failed: %w", err)
	}

	if u.Disabled {
		return &RedirectResult{
			RawURL: "",
			Status: 403,
		}, fmt.Errorf("url is disabled")
	}

	if u.MaxVisits() > 0 && u.Visits >= u.MaxVisits() {
		return &RedirectResult{
			RawURL: "",
			Status: 429,
		}, fmt.Errorf("max visits reached")
	}

	s.urlStore.IncrementVisits(req.Code, 1)

	if s.logStore != nil {
		_ = s.logStore.Log(store.AccessEntry{
			Code:      req.Code,
			RawURL:    u.RawURL,
			Timestamp: req.Timestamp,
		})
	}

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
