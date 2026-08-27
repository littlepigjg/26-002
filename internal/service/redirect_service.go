package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
)

// RedirectRequest is the request for a redirect.
type RedirectRequest struct {
	Code      string
	Timestamp time.Time
}

// RedirectResult is the result of a redirect.
type RedirectResult struct {
	RawURL string
	Status int
}

// RedirectService handles redirect operations.
type RedirectService struct {
	urlStore   *store.URLStore
	logStore   *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil || ls == nil {
		return nil, fmt.Errorf("url store and log store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect handles a redirect request.
func (r *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	url, err := r.urlStore.Get(req.Code)
	if err != nil {
		return nil, err
	}

	if url.IsExpired(req.Timestamp) {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, model.ErrDisabledURL
	}

	// Increment the live counter under the store's lock rather than
	// mutating the (copied) Get result, which would race and never reach
	// the stored record.
	if _, err := r.urlStore.IncrementVisits(req.Code); err != nil {
		return nil, err
	}

	_ = r.logStore.Write(store.AccessLogEntry{
		Code:      req.Code,
		Timestamp: req.Timestamp,
	})

	return &RedirectResult{
		RawURL: url.RawURL,
		Status: 302,
	}, nil
}

// BatchRedirect handles multiple redirects concurrently.
func (r *RedirectService) BatchRedirect(ctx context.Context, reqs []RedirectRequest) ([]*RedirectResult, error) {
	results := make([]*RedirectResult, len(reqs))
	for i, req := range reqs {
		result, err := r.HandleRedirect(ctx, &req)
		if err != nil {
			results[i] = &RedirectResult{
				Status: 500,
			}
		} else {
			results[i] = result
		}
	}
	return results, nil
}
