package ubaas_test

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

func TestDebugEvents(t *testing.T) {
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

	events := []*model.Event{
		makeEvent("userA", "s1", model.EventPageView, "/home", now.Add(0*time.Second)),
		makeEvent("userB", "s2", model.EventPageView, "/home", now.Add(1*time.Second)),
		makeEvent("userC", "s3", model.EventPageView, "/home", now.Add(2*time.Second)),
		makeEvent("userA", "s1", model.EventPageView, "/purchase", now.Add(3*time.Second)),
		makeEvent("userB", "s2", model.EventPageView, "/exit", now.Add(4*time.Second)),
		makeEvent("userC", "s3", model.EventPageView, "/purchase", now.Add(5*time.Second)),
	}
	memStore.CreateEvents(ctx, events)

	// Debug: ListEvents directly
	listed, total, err := memStore.ListEvents(ctx, model.EventQuery{
		StartDate: now.Add(-1 * time.Hour),
		EndDate:   now.Add(1 * time.Hour),
		Page:      1,
		PageSize:  50000,
	})
	fmt.Printf("ListEvents: %d total, %d returned, err=%v\n", total, len(listed), err)
	for _, e := range listed {
		fmt.Printf("  Event: ID=%s, UserID=%s, PageURL=%s, Type=%s, Timestamp=%v\n",
			e.ID, e.UserID, e.PageURL, e.Type, e.Timestamp)
	}

	query := model.ConversionQuery{
		StartPage: "/home",
		EndPage:   "/purchase",
		StartDate: now.Add(-1 * time.Hour),
		EndDate:   now.Add(1 * time.Hour),
	}

	result, err := svc.CalculateConversionRate(ctx, query)
	fmt.Printf("CalculateConversionRate result: %+v, err=%v\n", result, err)

	memStore.Close()
}

func makeEvent(userID, sessionID string, eventType model.EventType, pageURL string, ts time.Time) *model.Event {
	e := model.NewEvent(userID, sessionID, eventType, pageURL)
	e.Timestamp = ts
	e.CreatedAt = ts
	return e
}