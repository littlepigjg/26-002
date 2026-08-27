package service_test

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

// TestConversionQueryParamMatching reproduces bug #9: URLs with query/tracking
// parameters should match a conversion goal's bare start/end page.
//
// "/home?ref=banner", "/home?utm_source=google&campaign=test" must match the
// "/home" start page, and "/purchase?order_id=123&method=alipay" must match
// the "/purchase" end page.
func TestConversionQueryParamMatching(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelError, "test")
	memStore := store.NewMemoryStore(log)
	defer memStore.Close()
	cfg := config.DefaultConfig()
	svc := service.NewConversionService(memStore, cfg, log)

	ctx := context.Background()
	now := time.Now()

	goal := &model.ConversionGoal{
		ID:        "goal-query",
		StartPage: "/home",
		EndPage:   "/purchase",
	}
	if err := memStore.CreateConversionGoal(ctx, goal); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	events := []*model.Event{
		// userA: start page with tracking params -> purchase with order params
		makeEvent("userA", "s1", model.EventPageView, "/home?ref=banner", now.Add(0*time.Second)),
		makeEvent("userA", "s1", model.EventPageView, "/purchase?order_id=123&method=alipay", now.Add(1*time.Second)),
		// userB: start page with utm params -> no purchase
		makeEvent("userB", "s2", model.EventPageView, "/home?utm_source=google&campaign=test", now.Add(2*time.Second)),
		makeEvent("userB", "s2", model.EventPageView, "/exit", now.Add(3*time.Second)),
		// userC: bare paths (control case)
		makeEvent("userC", "s3", model.EventPageView, "/home", now.Add(4*time.Second)),
		makeEvent("userC", "s3", model.EventPageView, "/purchase", now.Add(5*time.Second)),
	}
	if err := memStore.CreateEvents(ctx, events); err != nil {
		t.Fatalf("create events: %v", err)
	}

	query := model.ConversionQuery{
		StartPage: "/home",
		EndPage:   "/purchase",
		StartDate: now.Add(-1 * time.Hour),
		EndDate:   now.Add(1 * time.Hour),
	}

	result, err := svc.CalculateConversionRate(ctx, query)
	if err != nil {
		t.Fatalf("calculate conversion rate: %v", err)
	}

	// All three users visited /home (in some parametrized form), so all three
	// should count as visitors, and userA + userC should be converted.
	if result.TotalVisitors != 3 {
		t.Fatalf("TotalVisitors = %d, want 3 (query-param URLs should match the bare start page)", result.TotalVisitors)
	}
	if result.ConvertedUsers != 2 {
		t.Fatalf("ConvertedUsers = %d, want 2 (userA and userC both reached /purchase)", result.ConvertedUsers)
	}
}

// TestConversionNoStrictModePollution reproduces the state-pollution half of
// bug #9: a request whose context forces strict mode must NOT leak that strict
// setting into later requests. Before the fix, once one request set the global
// strict flag, every subsequent request failed to match query-parameter URLs
// until the process was restarted.
func TestConversionNoStrictModePollution(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelError, "test")
	memStore := store.NewMemoryStore(log)
	defer memStore.Close()
	cfg := config.DefaultConfig()
	svc := service.NewConversionService(memStore, cfg, log)

	baseCtx := context.Background()
	now := time.Now()

	goal := &model.ConversionGoal{
		ID:        "goal-pollution",
		StartPage: "/home",
		EndPage:   "/purchase",
	}
	if err := memStore.CreateConversionGoal(baseCtx, goal); err != nil {
		t.Fatalf("create goal: %v", err)
	}

	events := []*model.Event{
		makeEvent("userA", "s1", model.EventPageView, "/home?ref=banner", now.Add(0*time.Second)),
		makeEvent("userA", "s1", model.EventPageView, "/purchase?order_id=123&method=alipay", now.Add(1*time.Second)),
	}
	if err := memStore.CreateEvents(baseCtx, events); err != nil {
		t.Fatalf("create events: %v", err)
	}

	query := model.ConversionQuery{
		StartPage: "/home",
		EndPage:   "/purchase",
		StartDate: now.Add(-1 * time.Hour),
		EndDate:   now.Add(1 * time.Hour),
	}

	// First call with a plain context should match the query-param URLs.
	first, err := svc.CalculateConversionRate(baseCtx, query)
	if err != nil {
		t.Fatalf("first call: %v", err)
	}
	if first.ConvertedUsers != 1 {
		t.Fatalf("first call: ConvertedUsers = %d, want 1", first.ConvertedUsers)
	}

	// Simulate a request whose context carries the strict-mode trigger value.
	// This request must not poison global state for subsequent requests.
	strictCtx := context.WithValue(baseCtx, "force_strict_url_match", "true")
	if _, err := svc.CalculateConversionRate(strictCtx, query); err != nil {
		t.Fatalf("strict call: %v", err)
	}

	// The package-global strict flag must be untouched by the strict request.
	if store.IsStrictURLCheck() {
		t.Fatalf("global strictURLCheck was mutated by a per-request context value; strict mode leaked across requests")
	}

	// A subsequent plain request must behave identically to the first one.
	second, err := svc.CalculateConversionRate(baseCtx, query)
	if err != nil {
		t.Fatalf("second call: %v", err)
	}
	if second.ConvertedUsers != 1 {
		t.Fatalf("after strict request, ConvertedUsers = %d, want 1 (global state was polluted by the strict request)", second.ConvertedUsers)
	}
}

// makeEvent is a local test helper that builds a page-view event with a fixed
// timestamp so conversion timing assertions are deterministic.
func makeEvent(userID, sessionID string, eventType model.EventType, pageURL string, ts time.Time) *model.Event {
	e := model.NewEvent(userID, sessionID, eventType, pageURL)
	e.Timestamp = ts
	e.CreatedAt = ts
	return e
}
