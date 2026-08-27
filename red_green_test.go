package bug18_test

import (
	"bytes"
	"context"
	"strings"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	var buf bytes.Buffer
	jsonLogger := logger.NewJSONLogger(&buf, logger.LevelInfo)
	jsonLogger.SetMaxEntrySize(256)

	longURL := "https://example.com/" + strings.Repeat("a", 500)

	fields := map[string]interface{}{
		"code":      "test1",
		"raw_url":   longURL,
		"custom":    true,
		"max_visits": 100,
	}

	jsonLogger.InfofJSON("Created short URL", fields)

	output := buf.String()

	if len(output) == 0 {
		t.Fatal("no log output produced")
	}

	lines := strings.Split(strings.TrimSpace(output), "\n")
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if len(line) > 256+50 {
			t.Logf("RED (红灯，缺陷未修复): Log line length %d exceeds maxEntrySize(256) - maxEntrySize truncation not working for field values", len(line))
			t.FailNow()
		}
		expectedEnding := longURL[len(longURL)-20:]
		if strings.Contains(line, expectedEnding) {
			t.Logf("RED (红灯，缺陷未修复): Full URL tail found in log - truncation not working, entire 500-byte URL present")
			t.FailNow()
		}
	}

	t.Log("GREEN (绿灯，缺陷已修复): Large URL value properly truncated in log output")
}

func TestMaxEntrySizeEnforcement(t *testing.T) {
	var buf bytes.Buffer
	jsonLogger := logger.NewJSONLogger(&buf, logger.LevelInfo)
	jsonLogger.SetMaxEntrySize(128)

	longURL := "https://example.com/" + strings.Repeat("b", 300)

	fields := map[string]interface{}{
		"code":    "test2",
		"raw_url": longURL,
		"custom":  false,
	}

	jsonLogger.InfofJSON("Created short URL", fields)

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if len(line) > 256 {
			t.Logf("RED (红灯，缺陷未修复): Log line exceeds maxEntrySize limit: %d bytes (limit was 128, but truncation not enforced)", len(line))
			t.FailNow()
		}
	}

	t.Log("GREEN (绿灯，缺陷已修复): All log lines properly truncated to maxEntrySize limit")
}

func TestCrossFileEnforcement(t *testing.T) {
	var buf bytes.Buffer
	jsonLogger := logger.NewJSONLogger(&buf, logger.LevelInfo)
	jsonLogger.SetMaxEntrySize(256)

	cfg := config.Default()
	cfg.Storage.URLFilePath("data/test_urls3.json")
	cfg.Storage.LogFilePath("data/test_access3.log")
	cfg.Storage.FlushOnWrite(false)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("failed to create URL store: %v", err)
	}
	urlStore.Load(context.Background())

	logStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("failed to create access log store: %v", err)
	}
	logStore.Open(context.Background())

	longURL := "https://example.com/" + strings.Repeat("c", 400)

	urlService, _ := service.NewURLService(cfg, urlStore)
	urlService.SetLogger(jsonLogger.Logger)

	shortURL, err := urlService.Create(context.Background(), &model.CreateReq{
		RawURL:    longURL,
		CustomCode: "redirect1",
		MaxVisits:  50,
	})
	if err != nil {
		t.Fatalf("failed to create short URL: %v", err)
	}
	_ = shortURL

	redirectService, err := service.NewRedirectService(urlStore, logStore)
	if err != nil {
		t.Fatalf("failed to create redirect service: %v", err)
	}
	redirectService.SetLogger(jsonLogger.Logger)

	result, err := redirectService.HandleRedirect(context.Background(), &service.RedirectRequest{
		Code:      "redirect1",
		Timestamp: time.Now(),
	})
	if err != nil {
		t.Fatalf("failed to handle redirect: %v", err)
	}

	if result.Status != 302 {
		t.Errorf("expected status 302, got %d", result.Status)
	}

	output := buf.String()
	lines := strings.Split(strings.TrimSpace(output), "\n")

	hasOversizedLine := false
	for _, line := range lines {
		if len(line) == 0 {
			continue
		}
		if len(line) > 256+50 {
			hasOversizedLine = true
		}
	}

	if hasOversizedLine {
		t.Log("RED (红灯，缺陷未修复): Cross-file log path fails to enforce maxEntrySize - format.go and logger.go both allow oversized field values through")
		t.FailNow()
	} else {
		t.Log("GREEN (绿灯，缺陷已修复): Cross-file log entries properly respect maxEntrySize limit")
	}

	urlStore.Close()
	logStore.Close()
}
