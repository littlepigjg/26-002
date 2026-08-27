package ubaas_test

import (
	"bytes"
	"context"
	"fmt"
	"testing"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	log := logger.New(bytes.NewBuffer(nil), logger.LevelError, "")
	memStore := store.NewMemoryStore(log)

	userID := "test-user-001"
	sessionID := "test-session-001"

	events := []*model.Event{
		model.NewEvent(userID, sessionID, model.EventPageView, "/page1"),
		model.NewEvent(userID, sessionID, model.EventClick, "/page2"),
		model.NewEvent(userID, sessionID, model.EventDuration, "/page3"),
	}

	memStore.SetContextCancelGuard(func(index int) bool {
		return index >= 2
	})

	ctx := context.Background()
	err := memStore.CreateEvents(ctx, events)

	if err != nil {
		createdCount := 0
		for _, e := range events {
			if _, getErr := memStore.GetEvent(ctx, e.ID); getErr == nil {
				createdCount++
			}
		}

		if createdCount > 0 {
			fmt.Println("RED (红灯，缺陷未修复)")
			t.Errorf("Expected 0 events stored when context cancelled, but got %d events persisted", createdCount)
		} else {
			fmt.Println("GREEN (绿灯，缺陷已修复)")
		}
	} else {
		createdCount := 0
		for _, e := range events {
			if _, getErr := memStore.GetEvent(ctx, e.ID); getErr == nil {
				createdCount++
			}
		}

		if createdCount == len(events) {
			fmt.Println("GREEN (绿灯，缺陷已修复)")
		} else {
			fmt.Println("RED (红灯，缺陷未修复)")
			t.Errorf("Expected %d events stored, but got %d", len(events), createdCount)
		}
	}
}
