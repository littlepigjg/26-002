package bug22_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URLStore: %v", err)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("failed to create AccessLogStore: %v", err)
	}

	urlService, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("failed to create URLService: %v", err)
	}

	redirectService, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("failed to create RedirectService: %v", err)
	}

	ctx := context.Background()

	shortURL, err := urlService.Create(ctx, &model.CreateReq{
		RawURL:     "https://example.com",
		CustomCode: "test1",
		MaxVisits:  100,
	})
	if err != nil {
		t.Fatalf("failed to create short URL: %v", err)
	}

	now := time.Now()

	_, err = redirectService.HandleRedirect(ctx, &service.RedirectRequest{
		Code:      shortURL.Code,
		Timestamp: now,
	})
	if err != nil {
		t.Fatalf("first redirect failed: %v", err)
	}

	_, err = redirectService.HandleRedirect(ctx, &service.RedirectRequest{
		Code:      shortURL.Code,
		Timestamp: now.Add(time.Second),
	})
	if err != nil {
		t.Fatalf("second redirect failed: %v", err)
	}

	refreshedURL, err := urlStore.Get(shortURL.Code)
	if err != nil {
		t.Fatalf("failed to get URL: %v", err)
	}

	logCount := logStore.Len()
	visitCount := refreshedURL.Visits

	if visitCount != logCount {
		fmt.Printf("RED: data inconsistency detected — URL visits=%d, access logs=%d\n", visitCount, logCount)
		t.Errorf("data inconsistency: URL has %d visits but only %d access log entries", visitCount, logCount)
	} else {
		fmt.Printf("GREEN: data consistent — URL visits=%d, access logs=%d\n", visitCount, logCount)
	}
}
