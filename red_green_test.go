package main

import (
	"context"
	"fmt"
	"runtime"
	"strings"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	cfg.Storage.URLFilePath("/tmp/test_urls.json")
	cfg.Storage.LogFilePath("/tmp/test_access.log")
	cfg.Storage.SyncInterval(5 * time.Second)
	cfg.Storage.FlushOnWrite(true)

	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore failed: %v", err)
	}
	defer us.Close()

	if err := us.Load(context.Background()); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore failed: %v", err)
	}
	defer ls.Close()

	if err := ls.Open(context.Background()); err != nil {
		t.Fatalf("AccessLogStore Open failed: %v", err)
	}

	urlSvc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService failed: %v", err)
	}

	redirSvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		t.Fatalf("NewRedirectService failed: %v", err)
	}

	for i := 0; i < 50; i++ {
		req := &model.CreateReq{
			RawURL:     fmt.Sprintf("https://example.com/page%d", i),
			CustomCode: fmt.Sprintf("code%03d", i),
			MaxVisits:  100,
		}
		_, err := urlSvc.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("Create failed for %d: %v", i, err)
		}
	}

	_, err = us.Get("code000")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}

	snapshot := us.RawSnapshot()
	if len(snapshot) != 50 {
		t.Fatalf("RawSnapshot expected 50, got %d", len(snapshot))
	}

	us.SetPanicGuard(func(code, rawURL string) bool {
		return false
	})

	redirectReq := &service.RedirectRequest{
		Code:      "code000",
		Timestamp: time.Now(),
	}
	result, err := redirSvc.HandleRedirect(context.Background(), redirectReq)
	if err != nil {
		t.Fatalf("HandleRedirect failed: %v", err)
	}
	if result.Status != 302 {
		t.Fatalf("Expected status 302, got %d", result.Status)
	}

	u, err := us.Get("code000")
	if err != nil {
		t.Fatalf("Get failed: %v", err)
	}
	if u.Validate() != nil {
		t.Fatal("Validate failed")
	}
	if u.IsExpired(time.Now()) {
		t.Fatal("Should not be expired")
	}

	if !strings.Contains(fmt.Sprint(cfg.Storage), "") {
		t.Fatal("Storage config not accessible")
	}

	goroutineBefore := runtime.NumGoroutine()

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Millisecond)
	defer cancel()

	_, err = us.SnapshotWithTimeout(ctx, 100*time.Millisecond)

	time.Sleep(200 * time.Millisecond)

	goroutineAfter := runtime.NumGoroutine()

	if goroutineAfter > goroutineBefore {
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Errorf("goroutine leak detected: before=%d, after=%d, diff=%d",
			goroutineBefore, goroutineAfter, goroutineAfter-goroutineBefore)
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	}
}
