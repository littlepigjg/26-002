package main

import (
	"context"
	"encoding/csv"
	"fmt"
	"path/filepath"
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
	model.ClearFieldOrder()

	tmpDir := t.TempDir()
	urlsPath := filepath.Join(tmpDir, "urls.json")
	logsPath := filepath.Join(tmpDir, "access.log")

	cfg := config.Default()
	cfg.Storage.URLFilePath(urlsPath)
	cfg.Storage.LogFilePath(logsPath)

	urlStore, err := store.NewURLStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create URL store: %v", err)
	}
	defer urlStore.Close()

	ctx := context.Background()
	if err := urlStore.Load(ctx); err != nil {
		t.Fatalf("Failed to load URL store: %v", err)
	}

	accessLogStore, err := store.NewAccessLogStore(cfg)
	if err != nil {
		t.Fatalf("Failed to create access log store: %v", err)
	}
	defer accessLogStore.Close()

	if err := accessLogStore.Open(ctx); err != nil {
		t.Fatalf("Failed to open access log store: %v", err)
	}

	urlService, err := service.NewURLService(cfg, urlStore)
	if err != nil {
		t.Fatalf("Failed to create URL service: %v", err)
	}

	createReq := &model.CreateReq{
		RawURL:     "https://example.com/very/long/path",
		CustomCode: "my-test",
		MaxVisits:  50,
	}

	shortURL, err := urlService.Create(ctx, createReq)
	if err != nil {
		t.Fatalf("Failed to create short URL: %v", err)
	}

	if shortURL.Code != "my-test" {
		t.Errorf("Expected code 'my-test', got '%s'", shortURL.Code)
	}
	if shortURL.RawURL != "https://example.com/very/long/path" {
		t.Errorf("Expected RawURL 'https://example.com/very/long/path', got '%s'", shortURL.RawURL)
	}
	if !shortURL.Custom {
		t.Error("Expected Custom to be true for custom code")
	}
	if shortURL.Disabled {
		t.Error("Expected Disabled to be false")
	}
	if shortURL.Visits != 0 {
		t.Errorf("Expected Visits 0, got %d", shortURL.Visits)
	}

	if err := shortURL.Validate(); err != nil {
		t.Errorf("ShortURL validation failed: %v", err)
	}
	if shortURL.IsExpired(time.Now()) {
		t.Error("ShortURL should not be expired")
	}
	if !shortURL.IsExpired(time.Now().Add(31 * 24 * time.Hour)) {
		t.Error("ShortURL should be expired after 31 days")
	}

	createReq2 := &model.CreateReq{
		RawURL: "https://another-example.com",
	}
	shortURL2, err := urlService.Create(ctx, createReq2)
	if err != nil {
		t.Fatalf("Failed to create second short URL: %v", err)
	}
	if shortURL2.Code == "" {
		t.Error("Expected non-empty auto-generated code")
	}
	if shortURL2.Custom {
		t.Error("Expected Custom to be false for auto-generated code")
	}

	snapshot := urlStore.RawSnapshot()
	if len(snapshot) < 2 {
		t.Errorf("Expected at least 2 URLs in snapshot, got %d", len(snapshot))
	}

	redirectService, err := service.NewRedirectService(urlStore, accessLogStore)
	if err != nil {
		t.Fatalf("Failed to create redirect service: %v", err)
	}

	redirectReq := &service.RedirectRequest{
		Code:      "my-test",
		Timestamp: time.Now(),
	}
	result, err := redirectService.HandleRedirect(ctx, redirectReq)
	if err != nil {
		t.Fatalf("Failed to handle redirect: %v", err)
	}
	if result.RawURL != "https://example.com/very/long/path" {
		t.Errorf("Expected RawURL 'https://example.com/very/long/path', got '%s'", result.RawURL)
	}
	if result.Status != 302 {
		t.Errorf("Expected Status 302, got %d", result.Status)
	}

	redirectReq2 := &service.RedirectRequest{
		Code:      "non-existent",
		Timestamp: time.Now(),
	}
	result2, err := redirectService.HandleRedirect(ctx, redirectReq2)
	if err != nil {
		t.Fatalf("Failed to handle redirect for non-existent code: %v", err)
	}
	if result2.Status != 404 {
		t.Errorf("Expected Status 404 for non-existent code, got %d", result2.Status)
	}

	model.ClearFieldOrder()

	memStore := store.NewMemoryStore(logger.New(nil, 2, "test"))
	defer memStore.Close()

	exportService := service.NewExportService(memStore, cfg, logger.New(nil, 2, "test"))

	testEvents := []*model.Event{
		{
			ID:         "evt-1",
			UserID:     "user-1",
			SessionID:  "ses-1",
			Type:       model.EventPageView,
			PageURL:    "/home",
			PageTitle:  "Home Page",
			DeviceType: model.DeviceDesktop,
			OS:         "Linux",
			Browser:    "Chrome",
			Country:    "US",
			Timestamp:  time.Now(),
			CreatedAt:  time.Now(),
		},
		{
			ID:         "evt-2",
			UserID:     "user-1",
			SessionID:  "ses-1",
			Type:       model.EventClick,
			PageURL:    "/products",
			PageTitle:  "Products",
			DeviceType: model.DeviceMobile,
			OS:         "iOS",
			Browser:    "Safari",
			Country:    "US",
			Timestamp:  time.Now(),
			CreatedAt:  time.Now(),
		},
	}

	for _, e := range testEvents {
		memStore.CreateEvent(ctx, e)
	}

	customFields := []string{"user_id", "id", "page_url"}
	query := model.EventQuery{
		Page:     1,
		PageSize: 100,
	}

	data, _, err := exportService.ExportEventsWithFields(ctx, query, model.ExportCSV, customFields)
	if err != nil {
		t.Fatalf("ExportEventsWithFields failed: %v", err)
	}

	reader := csv.NewReader(strings.NewReader(string(data)))
	records, err := reader.ReadAll()
	if err != nil {
		t.Fatalf("Failed to parse CSV: %v", err)
	}

	if len(records) < 2 {
		t.Fatalf("Expected at least 2 rows (header + data), got %d", len(records))
	}

	header := records[0]

	defectFound := false
	if len(header) < 3 || header[0] != "user_id" || header[1] != "id" || header[2] != "page_url" {
		defectFound = true
		t.Logf("DEFECT: Expected header to start with [%s %s %s], got [%s %s %s]",
			customFields[0], customFields[1], customFields[2],
			header[0], header[1], header[2])
	}

	model.ClearFieldOrder()

	validFields := []string{"id", "user_id", "page_url"}
	data2, _, err := exportService.ExportEventsWithFields(ctx, query, model.ExportCSV, validFields)
	if err != nil {
		t.Fatalf("ExportEventsWithFields (valid) failed: %v", err)
	}

	reader2 := csv.NewReader(strings.NewReader(string(data2)))
	records2, _ := reader2.ReadAll()
	if len(records2) >= 1 {
		if len(records2[0]) < 3 || records2[0][0] != "id" || records2[0][1] != "user_id" || records2[0][2] != "page_url" {
			defectFound = true
			t.Logf("DEFECT: Valid custom field order not applied correctly. Expected [%s %s %s], got [%s %s %s]",
				validFields[0], validFields[1], validFields[2],
				records2[0][0], records2[0][1], records2[0][2])
		}
	}

	data3, _, err := exportService.ExportEvents(ctx, query, model.ExportCSV)
	if err != nil {
		t.Fatalf("ExportEvents failed: %v", err)
	}

	reader3 := csv.NewReader(strings.NewReader(string(data3)))
	records3, _ := reader3.ReadAll()
	if len(records3) >= 1 {
		defaultFields := model.DefaultExportFields()
		header := records3[0]
		if len(header) != len(defaultFields) {
			defectFound = true
			t.Logf("DEFECT: State pollution detected. Expected %d default fields, got %d fields from cached order [%s %s %s]",
				len(defaultFields), len(header), header[0], header[1], header[2])
		}
	}

	if defectFound {
		fmt.Println("RED (红灯，缺陷未修复)")
		t.Fail()
	} else {
		fmt.Println("GREEN (绿灯，缺陷已修复)")
	}
}
