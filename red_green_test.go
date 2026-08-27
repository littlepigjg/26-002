package main

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
)

func TestRedGreen(t *testing.T) {
	cfg := config.Default()

	ctx := context.Background()

	storeCount := 10
	urlStores := make([]*store.URLStore, storeCount)
	svcList := make([]*service.URLService, storeCount)

	for i := 0; i < storeCount; i++ {
		s, err := store.NewURLStore(cfg)
		if err != nil {
			t.Fatalf("NewURLStore %d failed: %v", i, err)
		}
		urlStores[i] = s

		svc, err := service.NewURLService(cfg, s)
		if err != nil {
			t.Fatalf("NewURLService %d failed: %v", i, err)
		}
		svcList[i] = svc
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore failed: %v", err)
	}
	redirectSvc, err := service.NewRedirectService(urlStores[0], logStore)
	if err != nil {
		t.Fatalf("NewRedirectService failed: %v", err)
	}

	if err := urlStores[0].Load(ctx); err != nil {
		t.Fatalf("Load failed: %v", err)
	}

	const numWorkers = 60
	const numRounds = 8

	var wg sync.WaitGroup
	startCh := make(chan struct{})
	var panicCount int64
	var errCount int64
	var counterMu sync.Mutex

	worker := func(id int, svc *service.URLService) {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				counterMu.Lock()
				panicCount++
				counterMu.Unlock()
			}
		}()

		<-startCh

		for round := 0; round < numRounds; round++ {
			createCtx, cancel := context.WithTimeout(ctx, 500*time.Millisecond)
			_, createErr := svc.Create(createCtx, &model.CreateReq{
				RawURL:     fmt.Sprintf("http://example.com/path-%d", id),
				CustomCode: fmt.Sprintf("w%d-r%d", id, round),
				MaxVisits:  100,
			})
			cancel()
			if createErr != nil {
				counterMu.Lock()
				errCount++
				counterMu.Unlock()
			}
		}
	}

	updater := func(id int) {
		defer wg.Done()
		defer func() {
			if r := recover(); r != nil {
				counterMu.Lock()
				panicCount++
				counterMu.Unlock()
			}
		}()

		<-startCh

		for round := 0; round < numRounds; round++ {
			cfg.Update(func(c *config.Config) {
				idxMap := make(map[string]string, 50)
				for k := 0; k < 50; k++ {
					idxMap[fmt.Sprintf("u%d-r%d-k%d", id, round, k)] = fmt.Sprintf("v-%d", k)
				}
				c.Storage.SetURLIndex(idxMap)
			})
		}
	}

	for i := 0; i < numWorkers; i++ {
		wg.Add(1)
		go worker(i, svcList[i%storeCount])

		wg.Add(1)
		go updater(i)
	}

	close(startCh)
	wg.Wait()

	if panicCount > 0 {
		fmt.Printf("RED (红灯，缺陷未修复) - detected %d panics from concurrent map access\n", panicCount)
		t.FailNow()
	}

	snapshot := cfg.Get()
	idx := snapshot.Storage.GetURLIndex()
	if len(idx) == 0 {
		fmt.Println("RED (红灯，缺陷未修复) - URL index is empty after concurrent writes")
		t.FailNow()
	}

	shortURL, getErr := urlStores[0].Get("w0-r0")
	if getErr != nil || shortURL == nil {
		fmt.Printf("RED (红灯，缺陷未修复) - Get returned error or nil: %v\n", getErr)
		t.FailNow()
	}

	if shortURL.RawURL != "http://example.com/path-0" {
		fmt.Printf("RED (红灯，缺陷未修复) - unexpected RawURL: %s\n", shortURL.RawURL)
		t.FailNow()
	}

	result, redirectErr := redirectSvc.HandleRedirect(ctx, &service.RedirectRequest{
		Code:      "w0-r0",
		Timestamp: time.Now(),
	})
	if redirectErr != nil {
		fmt.Printf("RED (红灯，缺陷未修复) - redirect error: %v\n", redirectErr)
		t.FailNow()
	}
	if result.Status != 302 {
		fmt.Printf("RED (红灯，缺陷未修复) - expected 302, got %d\n", result.Status)
		t.FailNow()
	}

	snap2 := urlStores[0].RawSnapshot()
	if len(snap2) == 0 {
		fmt.Println("RED (红灯，缺陷未修复) - RawSnapshot returned empty map")
		t.FailNow()
	}

	for code := range snap2 {
		item, _ := urlStores[0].Get(code)
		if item == nil {
			fmt.Printf("RED (红灯，缺陷未修复) - Get returned nil for code %s\n", code)
			t.FailNow()
		}
		if item.RawURL == "" {
			fmt.Printf("RED (红灯，缺陷未修复) - empty RawURL for code %s\n", code)
			t.FailNow()
		}
	}

	fmt.Println("GREEN (绿灯，缺陷已修复)")
}
