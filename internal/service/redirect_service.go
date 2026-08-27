package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/store"
)

type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

type RedirectResult struct {
	RawURL string `json:"raw_url"`
	Status int    `json:"status"`
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
		return nil, fmt.Errorf("log store is required")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

func (r *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("request is nil")
	}

	u, err := r.urlStore.Get(req.Code)
	if err != nil {
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, nil
	}

	checkTime := req.Timestamp
	if checkTime.IsZero() {
		checkTime = time.Now()
	}

	if u.IsExpired(checkTime) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, nil
	}

	u.Visits++
	if err := r.urlStore.Save(u, true); err != nil {
		return nil, fmt.Errorf("failed to update url: %w", err)
	}

	if r.logStore != nil {
		r.logStore.Record(req.Code, 302, "", "")
	}

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
