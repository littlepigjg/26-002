package ubaas_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestConversionBugs(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelDebug, "test")
	memStore := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()
	svc := service.NewConversionService(memStore, cfg, log)

	ctx := context.Background()
	now := time.Now()

	goal := &model.ConversionGoal{
		ID:        "goal-test",
		StartPage: "/home",
		EndPage:   "/purchase",
	}
	memStore.CreateConversionGoal(ctx, goal)

	// Create events: 3 users view /home, 2 also view /purchase
	events := []*model.Event{
		newEventAt("userA", "s1", model.EventPageView, "/home", now.Add(0*time.Second)),
		newEventAt("userB", "s2", model.EventPageView, "/home", now.Add(1*time.Second)),
		newEventAt("userC", "s3", model.EventPageView, "/home", now.Add(2*time.Second)),
		newEventAt("userA", "s1", model.EventPageView, "/purchase", now.Add(3*time.Second)),
		newEventAt("userB", "s2", model.EventPageView, "/exit", now.Add(4*time.Second)),
		newEventAt("userC", "s3", model.EventPageView, "/purchase", now.Add(5*time.Second)),
	}
	memStore.CreateEvents(ctx, events)

	query := model.ConversionQuery{
		StartPage: "/home",
		EndPage:   "/purchase",
		StartDate: now.Add(-1 * time.Hour),
		EndDate:   now.Add(1 * time.Hour),
	}

	// ===== BUG 1: DropOffRate bug in CalculateConversionRate =====
	result, err := svc.CalculateConversionRate(ctx, query)
	if err != nil {
		t.Fatalf("CalculateConversionRate failed: %v", err)
	}

	expectedDropOffRate := 100.0 - result.ConversionRate
	t.Logf("ConversionRate=%.2f, DropOffRate=%.2f, ExpectedDropOffRate=%.2f",
		result.ConversionRate, result.DropOffRate, expectedDropOffRate)

	if result.DropOffRate != expectedDropOffRate {
		t.Errorf("RED: Bug 1 - DropOffRate should be %.2f but got %.2f (DropOffRate = ConversionRate instead of 100 - ConversionRate)",
			expectedDropOffRate, result.DropOffRate)
	} else {
		t.Logf("GREEN: Bug 1 fixed - DropOffRate is correct: %.2f", result.DropOffRate)
	}

	// ===== BUG 2: GetConversionSummary wrong visitor count =====
	summary, err := svc.GetConversionSummary(ctx, query)
	if err != nil {
		t.Fatalf("GetConversionSummary failed: %v", err)
	}

	t.Logf("Summary TotalVisitors=%d, ConvertedUsers=%d, ConversionRate=%.2f",
		summary.TotalVisitors, summary.ConvertedUsers, summary.ConversionRate)

	if summary.TotalVisitors != 3 {
		t.Errorf("RED: Bug 2 - GetConversionSummary TotalVisitors should be 3 but got %d (counts end-page viewers instead of start-page viewers)",
			summary.TotalVisitors)
	} else {
		t.Logf("GREEN: Bug 2 fixed - TotalVisitors is correct: %d", summary.TotalVisitors)
	}

	// ===== BUG 3: GetConversionSummary non-unique conversion counting =====
	events2 := []*model.Event{
		newEventAt("userX", "sx", model.EventPageView, "/home", now.Add(10*time.Second)),
		newEventAt("userX", "sx", model.EventPageView, "/home", now.Add(11*time.Second)),
		newEventAt("userX", "sx", model.EventPageView, "/home", now.Add(12*time.Second)),
		newEventAt("userX", "sx", model.EventPageView, "/purchase", now.Add(13*time.Second)),
		newEventAt("userY", "sy", model.EventPageView, "/home", now.Add(14*time.Second)),
		newEventAt("userY", "sy", model.EventPageView, "/exit", now.Add(15*time.Second)),
	}
	memStore.CreateEvents(ctx, events2)

	summary2, err := svc.GetConversionSummary(ctx, query)
	if err != nil {
		t.Fatalf("GetConversionSummary failed: %v", err)
	}

	t.Logf("Summary2 TotalVisitors=%d, ConvertedUsers=%d, ConversionRate=%.2f",
		summary2.TotalVisitors, summary2.ConvertedUsers, summary2.ConversionRate)

	if summary2.ConversionRate > 100.0 {
		t.Errorf("RED: Bug 3 - GetConversionSummary ConversionRate %.2f%% exceeds 100%% (non-unique conversion counting)",
			summary2.ConversionRate)
	} else {
		t.Logf("GREEN: Bug 3 fixed - ConversionRate is correct: %.2f%%", summary2.ConversionRate)
	}

	// ===== BUG 4: URL matching with query parameters broken by strict mode =====
	// Simulate a context that forces strict URL matching mode
	strictCtx := context.WithValue(ctx, "force_strict_url_match", "true")
	// Set the global strict URL check flag to simulate state pollution
	store.SetStrictURLCheck(true)

	// Create events with query parameters that should match the goal pages
	eventsWithParams := []*model.Event{
		newEventAt("userD", "s4", model.EventPageView, "/home?ref=banner", now.Add(20*time.Second)),
		newEventAt("userD", "s4", model.EventPageView, "/home?ref=banner&utm=ad", now.Add(21*time.Second)),
		newEventAt("userE", "s5", model.EventPageView, "/home?source=google", now.Add(22*time.Second)),
		newEventAt("userD", "s4", model.EventPageView, "/purchase?order=123", now.Add(23*time.Second)),
		newEventAt("userE", "s5", model.EventPageView, "/exit?reason=nav", now.Add(24*time.Second)),
	}
	memStore.CreateEvents(strictCtx, eventsWithParams)

	// Now calculate rate - the strict mode should break matching of URLs with query params
	// TotalVisitors should be 5 (userA, userB, userC, userD, userE all viewed /home or /home?)
	// Converted should be 3 (userA, userC, userD converted)

	result2, err := svc.CalculateConversionRate(strictCtx, query)
	if err != nil {
		t.Fatalf("CalculateConversionRate with params failed: %v", err)
	}

	t.Logf("Result with query params: Visitors=%d, Converted=%d, Rate=%.2f%%",
		result2.TotalVisitors, result2.ConvertedUsers, result2.ConversionRate)

	// With strict mode ON, URLs with query params like /home?ref=banner won't match /home
	// So userD and userE (who only viewed /home?ref=banner etc.) won't be counted
	// This means TotalVisitors will be lower than expected
	if result2.TotalVisitors >= 5 {
		t.Logf("GREEN: Bug 4 fixed - query parameter matching works correctly. TotalVisitors=%d (includes users with query params)", result2.TotalVisitors)
	} else {
		t.Errorf("RED: Bug 4 - URL matching too strict with query parameters. TotalVisitors=%d but should be >= 5. Users with query params are not being matched correctly.", result2.TotalVisitors)
	}

	// ===== BUG 5: Context state pollution - strict mode leaks to subsequent calls =====
	// After the above call, the global strict flag should be reset
	// But due to state pollution, it may still be active
	store.SetStrictURLCheck(false) // Reset for clean test

	events3 := []*model.Event{
		newEventAt("userF", "s6", model.EventPageView, "/home?campaign=test1", now.Add(30*time.Second)),
		newEventAt("userF", "s6", model.EventPageView, "/purchase?ref=checkout", now.Add(31*time.Second)),
	}
	memStore.CreateEvents(ctx, events3)

	result3, err := svc.CalculateConversionRate(ctx, query)
	if err != nil {
		t.Fatalf("CalculateConversionRate pollution test failed: %v", err)
	}

	t.Logf("Result after pollution: Visitors=%d, Converted=%d, Rate=%.2f%%",
		result3.TotalVisitors, result3.ConvertedUsers, result3.ConversionRate)

	// Should include userF who has query params in their URLs
	if result3.TotalVisitors >= 6 {
		t.Logf("GREEN: Bug 5 fixed - state pollution prevented. Query param URLs still match after context reset. TotalVisitors=%d", result3.TotalVisitors)
	} else {
		t.Errorf("RED: Bug 5 - Context state pollution causes URL matching failure. TotalVisitors=%d but should include userF with query params (>=6).", result3.TotalVisitors)
	}

	memStore.Close()
}

func newEventAt(userID, sessionID string, eventType model.EventType, pageURL string, ts time.Time) *model.Event {
	e := model.NewEvent(userID, sessionID, eventType, pageURL)
	e.Timestamp = ts
	e.CreatedAt = ts
	return e
}