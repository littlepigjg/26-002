package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/internal/service"
)

func TestRedGreen(t *testing.T) {
	hasFailure := false

	cfg := config.Default()
	us, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("NewURLStore error: %v", err)
	}

	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("NewAccessLogStore error: %v", err)
	}

	ctx := context.Background()

	if err := us.Load(ctx); err != nil {
		t.Errorf("URLStore.Load error: %v", err)
		hasFailure = true
	}

	if err := ls.Open(ctx); err != nil {
		t.Errorf("AccessLogStore.Open error: %v", err)
		hasFailure = true
	}

	urlSvc, err := service.NewURLService(cfg, us)
	if err != nil {
		t.Fatalf("NewURLService error: %v", err)
	}

	redirSvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		t.Fatalf("NewRedirectService error: %v", err)
	}

	// Test 1: 创建短链接后 Visits 应为 0
	req := &model.CreateReq{
		RawURL:    "https://example.com/page1",
		MaxVisits: 50,
	}
	shortURL, err := urlSvc.Create(ctx, req)
	if err != nil {
		t.Errorf("Create error: %v", err)
		hasFailure = true
	} else {
		if shortURL.Visits != 0 {
			t.Errorf("创建后 Visits 应为 0，实际为 %d", shortURL.Visits)
			hasFailure = true
		}
	}

	// Test 2: 从 store 获取短链接，Visits 应为 0
	if shortURL != nil {
		retrieved, err := us.Get(shortURL.Code)
		if err != nil {
			t.Errorf("Get error: %v", err)
			hasFailure = true
		} else if retrieved.Visits != 0 {
			t.Errorf("存储中的短链接 Visits 应为 0，实际为 %d", retrieved.Visits)
			hasFailure = true
		}
	}

	// Test 3: Get 返回的对象应与存储的一致（应返回副本，而非原始指针）
	if shortURL != nil {
		retrieved1, _ := us.Get(shortURL.Code)
		retrieved2, _ := us.Get(shortURL.Code)
		if retrieved1 == retrieved2 {
			t.Errorf("两次 Get 应返回不同的副本，实际返回了同一指针")
			hasFailure = true
		}
	}

	// Test 4: 覆盖保存（overwrite=true）应成功
	if shortURL != nil {
		modified := &model.ShortURL{
			Code:      shortURL.Code,
			RawURL:    "https://example.com/updated",
			CreatedAt: time.Now(),
			Visits:    0,
			Custom:    false,
			Disabled:  false,
		}
		err := us.Save(modified, true)
		if err != nil {
			t.Errorf("overwrite=true 保存应成功，实际返回错误: %v", err)
			hasFailure = true
		} else {
			updated, _ := us.Get(shortURL.Code)
			if updated != nil && updated.RawURL != "https://example.com/updated" {
				t.Errorf("覆盖后 RawURL 应更新，实际为 %s", updated.RawURL)
				hasFailure = true
			}
		}
	}

	// Test 5: 不覆盖保存（overwrite=false）应在代码已存在时返回错误
	if shortURL != nil {
		newURL := &model.ShortURL{
			Code:      shortURL.Code,
			RawURL:    "https://example.com/newwrong",
			CreatedAt: time.Now(),
			Visits:    0,
			Custom:    false,
			Disabled:  false,
		}
		err := us.Save(newURL, false)
		if err == nil {
			t.Errorf("overwrite=false 保存已存在的代码应返回错误，实际返回 nil")
			hasFailure = true
		}
	}

	// Test 6: 处理重定向后 Visits 应递增
	if shortURL != nil {
		_, _ = redirSvc.HandleRedirect(ctx, &service.RedirectRequest{
			Code:      shortURL.Code,
			Timestamp: time.Now(),
		})

		retrieved, _ := us.Get(shortURL.Code)
		if retrieved != nil && retrieved.Visits != 1 {
			t.Errorf("重定向后 Visits 应为 1，实际为 %d", retrieved.Visits)
			hasFailure = true
		}
	}

	// Test 7: context 取消后 Load 应返回错误
	cancelledCtx, cancel := context.WithCancel(context.Background())
	cancel()
	err = us.Load(cancelledCtx)
	if err == nil {
		t.Errorf("context 取消后 Load 应返回错误，实际返回 nil")
		hasFailure = true
	}

	// Test 8: SetPanicGuard 和 RawSnapshot 应正常工作
	urlSvc.SetPanicGuard(func(code, rawURL string) bool {
		return false
	})
	snapshot := urlSvc.RawSnapshot()
	if len(snapshot) < 1 {
		t.Errorf("RawSnapshot 应包含至少 1 条记录，实际为 %d", len(snapshot))
		hasFailure = true
	}

	// Test 9: BatchRedirect 正确处理多个请求
	if shortURL != nil {
		reqs := []service.RedirectRequest{
			{Code: shortURL.Code, Timestamp: time.Now()},
			{Code: shortURL.Code, Timestamp: time.Now()},
		}
		results, _ := redirSvc.BatchRedirect(ctx, reqs)
		if len(results) != 2 {
			t.Errorf("BatchRedirect 应返回 2 个结果，实际返回 %d", len(results))
			hasFailure = true
		}
		for i, r := range results {
			if r.Status != 302 {
				t.Errorf("BatchRedirect 第 %d 个结果 Status 应为 302，实际为 %d", i, r.Status)
				hasFailure = true
			}
		}
	}

	if hasFailure {
		fmt.Println("RED（红灯，缺陷未修复）")
		os.Exit(1)
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
		os.Exit(0)
	}
}
