package main

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/timeutil"
)

func TestRedGreen(t *testing.T) {
	timeutil.ResetGlobalState()

	cfg := config.Default()
	cfg.Storage.URLFilePath("/tmp/test_urls.json")
	cfg.Storage.LogFilePath("/tmp/test_access.log")
	cfg.Storage.SyncInterval(5 * time.Second)
	cfg.Storage.FlushOnWrite(false)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("无法创建 URLStore: %v", err)
	}

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("无法创建 AccessLogStore: %v", err)
	}

	if err := urlStore.Load(context.Background()); err != nil {
		t.Fatalf("URLStore 加载失败: %v", err)
	}

	if err := logStore.Open(context.Background()); err != nil {
		t.Fatalf("AccessLogStore 打开失败: %v", err)
	}

	urlService, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("无法创建 URLService: %v", err)
	}

	redirectService, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("无法创建 RedirectService: %v", err)
	}

	now := time.Now()
	hasDefect := false

	t.Run("创建时间戳准确性", func(t *testing.T) {
		timeutil.ResetGlobalState()

		req := &model.CreateReq{
			RawURL:     "https://example.com/test1",
			CustomCode: "test-code-1",
			MaxVisits:  100,
		}

		url, err := urlService.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("创建 URL 失败: %v", err)
		}

		timeDiff := url.CreatedAt.Sub(now)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}

		if timeDiff > 2*time.Hour {
			hasDefect = true
			t.Errorf("URL 创建时间戳偏差过大: 创建时=%v, 存储=%v, 偏差=%v (超过2小时阈值)",
				now.Format(time.RFC3339), url.CreatedAt.Format(time.RFC3339), timeDiff)
		} else {
			t.Logf("URL 创建时间戳在合理范围内 (偏差: %v)", timeDiff)
		}
	})

	t.Run("过期检查准确性", func(t *testing.T) {
		timeutil.ResetGlobalState()

		req := &model.CreateReq{
			RawURL:     "https://example.com/test2",
			CustomCode: "test-code-2",
			MaxVisits:  0,
		}

		url, err := urlService.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("创建 URL 失败: %v", err)
		}

		checkTime := url.CreatedAt.Add(25 * time.Hour)
		redirectReq := &service.RedirectRequest{
			Code:      url.Code,
			Timestamp: checkTime,
		}

		result, err := redirectService.HandleRedirect(context.Background(), redirectReq)
		if err != nil {
			t.Fatalf("HandleRedirect 返回错误: %v", err)
		}

		if result.Status != 410 {
			hasDefect = true
			t.Errorf("过期 URL 未被正确识别: 预期状态=410, 实际状态=%d, 创建时间=%v, 检查时间=%v",
				result.Status, url.CreatedAt.Format(time.RFC3339), checkTime.Format(time.RFC3339))
		} else {
			t.Logf("过期 URL 被正确识别 (状态: %d)", result.Status)
		}
	})

	t.Run("未过期 URL 可访问", func(t *testing.T) {
		timeutil.ResetGlobalState()

		req := &model.CreateReq{
			RawURL:     "https://example.com/test3",
			CustomCode: "test-code-3",
			MaxVisits:  0,
		}

		url, err := urlService.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("创建 URL 失败: %v", err)
		}

		immediateCheck := url.CreatedAt.Add(1 * time.Hour)
		redirectReq := &service.RedirectRequest{
			Code:      url.Code,
			Timestamp: immediateCheck,
		}

		result, err := redirectService.HandleRedirect(context.Background(), redirectReq)
		if err != nil {
			t.Fatalf("HandleRedirect 返回错误: %v", err)
		}

		if result.Status != 302 {
			hasDefect = true
			t.Errorf("未过期 URL 被错误标记: 预期状态=302, 实际状态=%d", result.Status)
		} else {
			t.Logf("未过期 URL 正常返回 (状态: %d)", result.Status)
		}
	})

	t.Run("存储数据一致性", func(t *testing.T) {
		timeutil.ResetGlobalState()

		req := &model.CreateReq{
			RawURL:     "https://example.com/test4",
			CustomCode: "test-code-4",
			MaxVisits:  50,
		}

		url, err := urlService.Create(context.Background(), req)
		if err != nil {
			t.Fatalf("创建 URL 失败: %v", err)
		}

		retrieved, err := urlStore.Get(url.Code)
		if err != nil {
			t.Fatalf("读取 URL 失败: %v", err)
		}

		if retrieved.Code != url.Code || retrieved.RawURL != url.RawURL {
			hasDefect = true
			t.Errorf("存储数据不一致: 原始 Code=%s RawURL=%s, 读取 Code=%s RawURL=%s",
				url.Code, url.RawURL, retrieved.Code, retrieved.RawURL)
		}

		timeDiff := retrieved.CreatedAt.Sub(now)
		if timeDiff < 0 {
			timeDiff = -timeDiff
		}
		if timeDiff > 2*time.Hour {
			hasDefect = true
			t.Errorf("存储时间戳偏差过大: 预期=%v, 实际=%v, 偏差=%v",
				now.Format(time.RFC3339), retrieved.CreatedAt.Format(time.RFC3339), timeDiff)
		} else {
			t.Logf("存储数据一致 (时间偏差: %v)", timeDiff)
		}
	})

	t.Run("禁用 URL 过期检查", func(t *testing.T) {
		disabledURL := &model.ShortURL{
			Code:      "disabled-code",
			RawURL:    "https://example.com/disabled",
			CreatedAt: time.Now().Add(-100 * time.Hour),
			Visits:    0,
			Custom:    false,
			Disabled:  true,
		}

		if err := urlStore.Save(disabledURL, true); err != nil {
			t.Fatalf("保存禁用 URL 失败: %v", err)
		}

		redirectReq := &service.RedirectRequest{
			Code:      "disabled-code",
			Timestamp: time.Now(),
		}

		result, err := redirectService.HandleRedirect(context.Background(), redirectReq)
		if err != nil {
			t.Fatalf("HandleRedirect 返回错误: %v", err)
		}

		if result.Status != 410 {
			hasDefect = true
			t.Errorf("禁用 URL 未被识别: 预期状态=410, 实际状态=%d", result.Status)
		} else {
			t.Logf("禁用 URL 正确返回过期状态 (状态: %d)", result.Status)
		}
	})

	fmt.Println()
	if hasDefect {
		fmt.Println("RED（红灯，缺陷未修复）")
	} else {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	}

	if hasDefect {
		os.Exit(1)
	}
}
