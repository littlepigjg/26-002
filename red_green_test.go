package bug19_test

import (
	"context"
	"fmt"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/internal/service"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	// Create URL store
	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create URL store: %v", err)
	}

	// Create access log store
	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create access log store: %v", err)
	}

	// Open log store
	ctx := context.Background()
	if err := logStore.Open(ctx); err != nil {
		t.Fatalf("Failed to open log store: %v", err)
	}

	// Create URL service
	urlSvc, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("Failed to create URL service: %v", err)
	}

	// Create redirect service
	redirectSvc, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("Failed to create redirect service: %v", err)
	}

	// Test 1: Create and access a single short URL
	t.Run("CreateAndAccessSingleURL", func(t *testing.T) {
		req := &model.CreateReq{
			RawURL:    "https://example.com/page1",
			CustomCode: "test1",
			MaxVisits:  100,
		}

		shortURL, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Errorf("GREEN (绿灯，缺陷已修复)")
			t.Fatalf("Failed to create short URL: %v", err)
		}

		if shortURL.Code != "test1" {
			t.Errorf("Expected code 'test1', got '%s'", shortURL.Code)
		}

		// Try to access the URL
		result, err := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
			Code:      "test1",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Errorf("RED (红灯，缺陷未修复)")
			t.Fatalf("Failed to handle redirect: %v", err)
		}

		if result.Status != 302 {
			t.Errorf("Expected status 302, got %d", result.Status)
		}

		if result.RawURL != "https://example.com/page1" {
			t.Errorf("Expected raw URL 'https://example.com/page1', got '%s'", result.RawURL)
		}

		fmt.Println("GREEN (绿灯，缺陷已修复)")
	})

	// Test 2: Create and access multiple short URLs
	t.Run("CreateAndAccessMultipleURLs", func(t *testing.T) {
		// Create first URL
		req1 := &model.CreateReq{
			RawURL:    "https://example.com/page2",
			CustomCode: "test2",
			MaxVisits:  100,
		}

		_, err := urlSvc.Create(ctx, req1)
		if err != nil {
			t.Errorf("Failed to create first short URL: %v", err)
		}

		// Create second URL
		req2 := &model.CreateReq{
			RawURL:    "https://example.com/page3",
			CustomCode: "test3",
			MaxVisits:  100,
		}

		_, err = urlSvc.Create(ctx, req2)
		if err != nil {
			t.Errorf("Failed to create second short URL: %v", err)
		}

		// Access first URL
		result1, err := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
			Code:      "test2",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Errorf("RED (红灯，缺陷未修复)")
			t.Fatalf("Failed to access first URL: %v", err)
		}

		if result1.RawURL != "https://example.com/page2" {
			t.Errorf("Expected raw URL 'https://example.com/page2', got '%s'", result1.RawURL)
		}

		// Access second URL
		result2, err := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
			Code:      "test3",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Errorf("RED (红灯，缺陷未修复)")
			t.Fatalf("Failed to access second URL: %v", err)
		}

		if result2.RawURL != "https://example.com/page3" {
			t.Errorf("Expected raw URL 'https://example.com/page3', got '%s'", result2.RawURL)
		}

		fmt.Println("GREEN (绿灯，缺陷已修复)")
	})

	// Test 3: Access same URL multiple times
	t.Run("AccessSameURLMultipleTimes", func(t *testing.T) {
		req := &model.CreateReq{
			RawURL:    "https://example.com/page4",
			CustomCode: "test4",
			MaxVisits:  100,
		}

		_, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Errorf("Failed to create short URL: %v", err)
		}

		// Access URL multiple times
		for i := 0; i < 10; i++ {
			result, err := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
				Code:      "test4",
				Timestamp: time.Now(),
			})
			if err != nil {
				t.Errorf("RED (红灯，缺陷未修复)")
				t.Fatalf("Failed to access URL on iteration %d: %v", i, err)
			}

			if result.Status != 302 {
				t.Errorf("Expected status 302 on iteration %d, got %d", i, result.Status)
			}
		}

		fmt.Println("GREEN (绿灯，缺陷已修复)")
	})

	// Test 4: Verify diagnostic snapshot and access
	t.Run("VerifyDiagnosticSnapshot", func(t *testing.T) {
		req := &model.CreateReq{
			RawURL:    "https://example.com/page5",
			CustomCode: "test5",
			MaxVisits:  100,
		}

		_, err := urlSvc.Create(ctx, req)
		if err != nil {
			t.Errorf("Failed to create short URL: %v", err)
		}

		// Verify URL is in snapshot
		snapshot := urlStore.RawSnapshot()
		if len(snapshot) == 0 {
			t.Errorf("RED (红灯，缺陷未修复)")
			t.Fatal("Snapshot is empty")
		}

		if _, ok := snapshot["test5"]; !ok {
			t.Errorf("RED (红灯，缺陷未修复)")
			t.Fatalf("URL 'test5' not found in snapshot")
		}

		// Access the URL - should succeed but will fail due to defect
		result, err := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
			Code:      "test5",
			Timestamp: time.Now(),
		})
		if err != nil {
			t.Errorf("RED (红灯，缺陷未修复)")
			t.Fatalf("Failed to access URL: %v", err)
		}

		if result.Status != 302 {
			t.Errorf("Expected status 302, got %d", result.Status)
		}

		if result.RawURL != "https://example.com/page5" {
			t.Errorf("Expected raw URL 'https://example.com/page5', got '%s'", result.RawURL)
		}

		fmt.Println("GREEN (绿灯，缺陷已修复)")
	})

	// Cleanup
	urlStore.Close()
	logStore.Close()
}
