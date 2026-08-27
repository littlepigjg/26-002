package main

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/handler"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

type testStore struct {
	store.MemoryStore
	fixedEvents []*model.Event
}

func (ts *testStore) ListEvents(ctx context.Context, query model.EventQuery) ([]*model.Event, int, error) {
	return ts.fixedEvents, len(ts.fixedEvents), nil
}

func TestRedGreen(t *testing.T) {
	cfg := config.Default()
	log := logger.New(os.Stdout, logger.LevelInfo, "test")
	memStore := store.NewMemoryStore(log)
	memStore.Close()

	events := []*model.Event{
		{ID: "1", DeviceType: model.DeviceDesktop, OS: "windows", Browser: "chrome", Country: "US", Timestamp: time.Now()},
		{ID: "2", DeviceType: model.DeviceMobile, OS: "android", Browser: "safari", Country: "CN", Timestamp: time.Now()},
		{ID: "3", DeviceType: model.DeviceDesktop, OS: "mac", Browser: "firefox", Country: "US", Timestamp: time.Now()},
	}

	ts := &testStore{fixedEvents: events}

	svc := service.NewDimensionService(ts, cfg, log)

	t.Run("OR logic returns events matching ANY condition", func(t *testing.T) {
		req := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimDeviceType, Operator: model.OpEqual, Value: string(model.DeviceDesktop)},
				{Dimension: model.DimCountry, Operator: model.OpEqual, Value: "CN"},
			},
			Logic: model.LogicOr,
		}

		result, err := svc.ApplyFilters(context.Background(), req)
		if err != nil {
			t.Fatalf("ApplyFilters returned error: %v", err)
		}

		if result.TotalCount == 3 {
			t.Logf("GREEN (绿灯，缺陷已修复): OR logic returned %d events (desktop OR CN = events 1,2,3)", result.TotalCount)
			return
		}
		t.Logf("RED (红灯，缺陷未修复): OR logic returned %d events, expected 3 (desktop OR CN)", result.TotalCount)
		t.Fail()
	})

	t.Run("AND logic returns events matching ALL conditions", func(t *testing.T) {
		req := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimDeviceType, Operator: model.OpEqual, Value: string(model.DeviceDesktop)},
				{Dimension: model.DimCountry, Operator: model.OpEqual, Value: "US"},
			},
			Logic: model.LogicAnd,
		}

		result, err := svc.ApplyFilters(context.Background(), req)
		if err != nil {
			t.Fatalf("ApplyFilters returned error: %v", err)
		}

		if result.TotalCount == 2 {
			t.Logf("GREEN (绿灯，缺陷已修复): AND logic returned %d events (desktop AND US = events 1,3)", result.TotalCount)
			return
		}
		t.Logf("RED (红灯，缺陷未修复): AND logic returned %d events, expected 2 (desktop AND US)", result.TotalCount)
		t.Fail()
	})

	t.Run("single OR condition returns correct count", func(t *testing.T) {
		req := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimBrowser, Operator: model.OpEqual, Value: "chrome"},
				{Dimension: model.DimBrowser, Operator: model.OpEqual, Value: "firefox"},
			},
			Logic: model.LogicOr,
		}

		result, err := svc.ApplyFilters(context.Background(), req)
		if err != nil {
			t.Fatalf("ApplyFilters returned error: %v", err)
		}

		if result.TotalCount == 2 {
			t.Logf("GREEN (绿灯，缺陷已修复): OR with 2 browser conditions returned %d events", result.TotalCount)
			return
		}
		t.Logf("RED (红灯，缺陷未修复): OR with 2 browser conditions returned %d events, expected 2", result.TotalCount)
		t.Fail()
	})

	t.Run("public API contract symbols", func(t *testing.T) {
		cfg := config.Default()
		if cfg == nil {
			t.Error("config.Default() returned nil")
		}
		us, err := store.NewURLStore(cfg)
		if err != nil {
			t.Errorf("NewURLStore failed: %v", err)
		}
		if err := us.Load(context.Background()); err != nil {
			t.Errorf("URLStore.Load failed: %v", err)
		}
		us.SetPanicGuard(func(code, rawURL string) bool { return true })
		snap := us.RawSnapshot()
		if snap == nil {
			t.Error("URLStore.RawSnapshot returned nil map")
		}
		_, err = service.NewURLService(cfg, us)
		if err != nil {
			t.Errorf("NewURLService failed: %v", err)
		}
		ls, err := store.NewAccessLogStore(cfg)
		if err != nil {
			t.Errorf("NewAccessLogStore failed: %v", err)
		}
		if err := ls.Open(context.Background()); err != nil {
			t.Errorf("AccessLogStore.Open failed: %v", err)
		}
		_ = ls.Close()
		_ = us.Close()
		if t.Failed() {
			t.Logf("RED (红灯，缺陷未修复): public API contract symbols check failed")
			t.FailNow()
		}
		t.Logf("GREEN (绿灯，缺陷已修复): public API contract symbols all exist and compile")
	})

	t.Run("handler compilation works", func(t *testing.T) {
		h := handler.NewDimensionHandler(svc, log)
		if h == nil {
			t.Error("NewDimensionHandler returned nil")
		}
		if t.Failed() {
			t.Logf("RED (红灯，缺陷未修复): handler compilation check failed")
			t.FailNow()
		}
		t.Logf("GREEN (绿灯，缺陷已修复): handler compiles")
	})

	t.Run("OpContains operator matches substring", func(t *testing.T) {
		req := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimBrowser, Operator: model.OpContains, Value: "chr"},
			},
			Logic: model.LogicAnd,
		}

		result, err := svc.ApplyFilters(context.Background(), req)
		if err != nil {
			t.Fatalf("ApplyFilters returned error: %v", err)
		}

		if result.TotalCount == 1 {
			t.Logf("GREEN (绿灯，缺陷已修复): OpContains returned %d events (chrome contains 'chr')", result.TotalCount)
			return
		}
		t.Logf("RED (红灯，缺陷未修复): OpContains returned %d events, expected 1", result.TotalCount)
		t.Fail()
	})

	t.Run("Load works with any context", func(t *testing.T) {
		cfg2 := config.Default()
		us2, err := store.NewURLStore(cfg2)
		if err != nil {
			t.Fatalf("NewURLStore failed: %v", err)
		}
		if err := us2.Load(context.Background()); err != nil {
			t.Errorf("URLStore.Load with background failed: %v", err)
		}
		_ = us2.Close()
		if t.Failed() {
			t.Logf("RED (红灯，缺陷未修复): Load should work with any context")
			t.FailNow()
		}
		t.Logf("GREEN (绿灯，缺陷已修复): Load works with any context")
	})

	t.Run("RawSnapshot returns empty map for new store", func(t *testing.T) {
		cfg3 := config.Default()
		us3, err := store.NewURLStore(cfg3)
		if err != nil {
			t.Fatalf("NewURLStore failed: %v", err)
		}
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := us3.Load(ctx); err != nil {
			t.Fatalf("URLStore.Load failed: %v", err)
		}
		snap := us3.RawSnapshot()
		if snap == nil {
			t.Errorf("RawSnapshot returned nil for initialized store")
		}
		_ = us3.Close()
		if t.Failed() {
			t.Logf("RED (红灯，缺陷未修复): RawSnapshot should return empty map, not nil")
			t.FailNow()
		}
		t.Logf("GREEN (绿灯，缺陷已修复): RawSnapshot returns empty map")
	})

	t.Run("handler returns errors via HTTP", func(t *testing.T) {
		cfg2 := config.Default()
		log2 := logger.New(os.Stdout, logger.LevelInfo, "test-http")
		memStore2 := store.NewMemoryStore(log2)
		defer memStore2.Close()

		eventSvc := service.NewEventService(memStore2, cfg2, log2)
		sessionSvc := service.NewSessionService(memStore2, cfg2, log2)
		pathSvc := service.NewPathService(memStore2, cfg2, log2)
		statsSvc := service.NewStatsService(memStore2, cfg2, log2)
		convSvc := service.NewConversionService(memStore2, cfg2, log2)
		dimSvc2 := service.NewDimensionService(memStore2, cfg2, log2)
		exportSvc := service.NewExportService(memStore2, cfg2, log2)

		router := handler.NewAPIRouter(eventSvc, sessionSvc, pathSvc, statsSvc, convSvc, dimSvc2, exportSvc)
		server := httptest.NewServer(router.Handler())
		defer server.Close()

		body := `{"conditions":[{"dimension":"device_type","operator":"eq","value":"desktop"},{"dimension":"country","operator":"eq","value":"US"}],"logic":"and"}`
		resp, err := http.Post(server.URL+"/api/dimensions/filter", "application/json", strings.NewReader(body))
		if err != nil {
			t.Fatalf("HTTP request failed: %v", err)
		}
		defer resp.Body.Close()

		var result struct {
			Code    int                    `json:"code"`
			Message string                 `json:"message"`
			Data    map[string]interface{} `json:"data"`
		}
		json.NewDecoder(resp.Body).Decode(&result)

		if result.Code != 0 {
			t.Errorf("Handler returned error: code=%d msg=%s", result.Code, result.Message)
		}
		if t.Failed() {
			t.Logf("RED (红灯，缺陷未修复): handler request failed (code=%d)", result.Code)
			t.FailNow()
		}
		t.Logf("GREEN (绿灯，缺陷已修复): handler returned success (code=%d)", result.Code)
	})

	fmt.Println("All tests completed")
}
