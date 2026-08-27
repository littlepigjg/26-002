package ubaas

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
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelError, "")
	cfg := config.DefaultConfig()
	memStore := store.NewMemoryStore(log)

	svc := service.NewDimensionService(memStore, cfg, log)
	ctx := context.Background()

	// Create test events with mixed case values
	now := time.Now()
	events := []*model.EventCreateRequest{
		{
			UserID:     "user1",
			SessionID:  "session1",
			Type:       model.EventPageView,
			PageURL:    "/home",
			DeviceType: model.DeviceDesktop,
			OS:         "Windows 10",
			Browser:    "Chrome",
			Country:    "US",
			Timestamp:  now,
		},
		{
			UserID:     "user1",
			SessionID:  "session1",
			Type:       model.EventPageView,
			PageURL:    "/about",
			DeviceType: model.DeviceDesktop,
			OS:         "Windows 11",
			Browser:    "Chrome",
			Country:    "US",
			Timestamp:  now,
		},
		{
			UserID:     "user2",
			SessionID:  "session2",
			Type:       model.EventPageView,
			PageURL:    "/contact",
			DeviceType: model.DeviceMobile,
			OS:         "Linux",
			Browser:    "Firefox",
			Country:    "CN",
			Timestamp:  now,
		},
	}

	// Store events
	for _, req := range events {
		event := req.ToEvent()
		memStore.CreateEvent(ctx, event)
	}

	allPassed := true

	// Test 1: Store-level filter with matching case should return event
	t.Run("StoreFilter_MatchingCase", func(t *testing.T) {
		query := model.EventQuery{
			OS:       "Windows 10",
			Page:     1,
			PageSize: 50,
		}
		storeEvents, _, err := memStore.ListEvents(ctx, query)
		if err != nil {
			t.Errorf("Test 1 failed with error: %v", err)
			allPassed = false
			return
		}
		if len(storeEvents) != 1 {
			t.Errorf("Test 1: Store filter with OS='Windows 10' should return 1 event, got %d", len(storeEvents))
			allPassed = false
		}
	})

	// Test 2: Store-level filter with different case should NOT return event (case-sensitive)
	t.Run("StoreFilter_DifferentCase", func(t *testing.T) {
		query := model.EventQuery{
			OS:       "windows 10",
			Page:     1,
			PageSize: 50,
		}
		storeEvents, _, err := memStore.ListEvents(ctx, query)
		if err != nil {
			t.Errorf("Test 2 failed with error: %v", err)
			allPassed = false
			return
		}
		if len(storeEvents) != 0 {
			t.Errorf("Test 2: Store filter with OS='windows 10' should return 0 events (case-sensitive), got %d", len(storeEvents))
			allPassed = false
		}
	})

	// Test 3: Service-level filter with matching case should return event
	t.Run("ServiceFilter_MatchingCase", func(t *testing.T) {
		req := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimOS, Operator: model.OpEqual, Value: "Windows 10"},
			},
			Logic: model.LogicAnd,
		}
		result, err := svc.ApplyFilters(ctx, req)
		if err != nil {
			t.Errorf("Test 3 failed with error: %v", err)
			allPassed = false
			return
		}
		if result.TotalCount != 1 {
			t.Errorf("Test 3: Service filter with OS='Windows 10' should return 1 event, got %d", result.TotalCount)
			allPassed = false
		}
	})

	// Test 4: Service-level filter with different case should NOT return event (case-sensitive)
	t.Run("ServiceFilter_DifferentCase", func(t *testing.T) {
		req := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimOS, Operator: model.OpEqual, Value: "windows 10"},
			},
			Logic: model.LogicAnd,
		}
		result, err := svc.ApplyFilters(ctx, req)
		if err != nil {
			t.Errorf("Test 4 failed with error: %v", err)
			allPassed = false
			return
		}
		if result.TotalCount != 0 {
			t.Errorf("Test 4: Service filter with OS='windows 10' should return 0 events (case-sensitive), got %d", result.TotalCount)
			allPassed = false
		}
	})

	// Test 5: Consistency check - store and service should return same results for same filter
	t.Run("ConsistencyCheck_OS", func(t *testing.T) {
		// Test with matching case
		storeQuery := model.EventQuery{OS: "Windows 10", Page: 1, PageSize: 50}
		storeEvents, _, _ := memStore.ListEvents(ctx, storeQuery)

		svcReq := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimOS, Operator: model.OpEqual, Value: "Windows 10"},
			},
			Logic: model.LogicAnd,
		}
		svcResult, _ := svc.ApplyFilters(ctx, svcReq)

		if len(storeEvents) != svcResult.TotalCount {
			t.Errorf("Test 5a: Inconsistency - store returned %d, service returned %d for OS='Windows 10'",
				len(storeEvents), svcResult.TotalCount)
			allPassed = false
		}

		// Test with different case
		storeQuery2 := model.EventQuery{OS: "windows 10", Page: 1, PageSize: 50}
		storeEvents2, _, _ := memStore.ListEvents(ctx, storeQuery2)

		svcReq2 := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimOS, Operator: model.OpEqual, Value: "windows 10"},
			},
			Logic: model.LogicAnd,
		}
		svcResult2, _ := svc.ApplyFilters(ctx, svcReq2)

		if len(storeEvents2) != svcResult2.TotalCount {
			t.Errorf("Test 5b: Inconsistency - store returned %d, service returned %d for OS='windows 10'",
				len(storeEvents2), svcResult2.TotalCount)
			allPassed = false
		}
	})

	// Test 6: Consistency check for Browser filtering
	t.Run("ConsistencyCheck_Browser", func(t *testing.T) {
		storeQuery := model.EventQuery{Browser: "Chrome", Page: 1, PageSize: 50}
		storeEvents, _, _ := memStore.ListEvents(ctx, storeQuery)

		svcReq := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimBrowser, Operator: model.OpEqual, Value: "Chrome"},
			},
			Logic: model.LogicAnd,
		}
		svcResult, _ := svc.ApplyFilters(ctx, svcReq)

		if len(storeEvents) != svcResult.TotalCount {
			t.Errorf("Test 6: Inconsistency - store returned %d, service returned %d for Browser='Chrome'",
				len(storeEvents), svcResult.TotalCount)
			allPassed = false
		}
	})

	// Test 7: Consistency check for Country filtering
	t.Run("ConsistencyCheck_Country", func(t *testing.T) {
		storeQuery := model.EventQuery{Country: "US", Page: 1, PageSize: 50}
		storeEvents, _, _ := memStore.ListEvents(ctx, storeQuery)

		svcReq := &model.FilterRequest{
			Conditions: []model.FilterCondition{
				{Dimension: model.DimCountry, Operator: model.OpEqual, Value: "US"},
			},
			Logic: model.LogicAnd,
		}
		svcResult, _ := svc.ApplyFilters(ctx, svcReq)

		if len(storeEvents) != svcResult.TotalCount {
			t.Errorf("Test 7: Inconsistency - store returned %d, service returned %d for Country='US'",
				len(storeEvents), svcResult.TotalCount)
			allPassed = false
		}
	})

	// Test 8: QueryService consistency check
	t.Run("QueryService_ConsistencyCheck", func(t *testing.T) {
		qsvc := service.NewQueryService(memStore, cfg, log)

		// Direct store query
		storeQuery := model.EventQuery{OS: "Windows 10", Page: 1, PageSize: 50}
		storeEvents, _, _ := memStore.ListEvents(ctx, storeQuery)

		// QueryService query with same filter
		qreq := service.QueryRequest{
			Filters: []model.FilterCondition{
				{Dimension: model.DimOS, Operator: model.OpEqual, Value: "Windows 10"},
			},
			Limit:  50,
			Offset: 0,
		}
		qresult, err := qsvc.ExecuteQuery(ctx, qreq)
		if err != nil {
			t.Errorf("Test 8 QueryService failed: %v", err)
			allPassed = false
			return
		}

		resultCount := 0
		if data, ok := qresult.Data.([]*model.Event); ok {
			resultCount = len(data)
		}

		if len(storeEvents) != resultCount {
			t.Errorf("Test 8: Inconsistency - store returned %d, QueryService returned %d for OS='Windows 10'",
				len(storeEvents), resultCount)
			allPassed = false
		}
	})

	if allPassed {
		fmt.Println("GREEN（绿灯，缺陷已修复）")
	} else {
		fmt.Println("RED（红灯，缺陷未修复）")
		t.Fatal("Filter inconsistency detected - RED (defect not fixed)")
	}
}
