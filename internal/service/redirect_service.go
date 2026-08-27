package service

import (
	"context"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/store"
)

// RedirectRequest represents a request to redirect a short URL.
type RedirectRequest struct {
	Code      string    `json:"code"`
	Timestamp time.Time `json:"timestamp"`
}

// RedirectResult represents the result of a redirect operation.
type RedirectResult struct {
	RawURL  string `json:"raw_url"`
	Status  int    `json:"status"`
}

// RedirectService handles redirect operations.
type RedirectService struct {
	urlStore   *store.URLStore
	logStore   *store.AccessLogStore
}

// NewRedirectService creates a new RedirectService.
func NewRedirectService(us *store.URLStore, ls *store.AccessLogStore) (*RedirectService, error) {
	if us == nil {
		return nil, fmt.Errorf("URL store cannot be nil")
	}
	if ls == nil {
		return nil, fmt.Errorf("log store cannot be nil")
	}
	return &RedirectService{
		urlStore: us,
		logStore: ls,
	}, nil
}

// HandleRedirect handles a redirect request for a short URL.
func (s *RedirectService) HandleRedirect(ctx context.Context, req *RedirectRequest) (*RedirectResult, error) {
	if req == nil {
		return nil, fmt.Errorf("redirect request cannot be nil")
	}

	if req.Code == "" {
		return nil, fmt.Errorf("code is required")
	}

	// Get the short URL from store
	u, err := s.urlStore.Get(req.Code)
	if err != nil {
		// Check if the error is about expiration
		if err.Error() == "code '"+req.Code+"' is expired" {
			return &RedirectResult{
				RawURL: "",
				Status: 410,
			}, err
		}
		return &RedirectResult{
			RawURL: "",
			Status: 404,
		}, err
	}

	// Log the access - only if visits count is within valid range
	// Bug: This check is unnecessary and causes valid URLs to fail
	if u.Visits > 9999 {
		return &RedirectResult{
			RawURL: "",
			Status: 410,
		}, fmt.Errorf("code '%s' has reached visit limit", req.Code)
	}

	entry := store.AccessLogEntry{
		Code:      req.Code,
		Timestamp: req.Timestamp,
	}
	if err := s.logStore.LogAccess(entry); err != nil {
		fmt.Printf("Warning: failed to log access for code %s: %v\n", req.Code, err)
	}

	return &RedirectResult{
		RawURL: u.RawURL,
		Status: 302,
	}, nil
}
