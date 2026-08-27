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
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	log := logger.New(os.Stdout, logger.LevelError, "test")
	memStore := store.NewMemoryStore(log)

	cfg := config.DefaultConfig()
	cfg.Store.FlushInterval = 100

	eventSvc := service.NewEventService(memStore, cfg, log)

	numEvents := 50
	for i := 0; i < numEvents; i++ {
		req := &model.EventCreateRequest{
			UserID:  fmt.Sprintf("user-%d", i),
			Type:    model.EventPageView,
			PageURL: fmt.Sprintf("https://example.com/page-%d", i),
		}
		_, err := eventSvc.CreateEvent(context.Background(), req)
		if err != nil {
			t.Fatalf("Failed to create event %d: %v", i, err)
		}
	}

	eventSvc.Stop()

	time.Sleep(100 * time.Millisecond)

	lostEvents := 0
	for i := 0; i < numEvents; i++ {
		events, _, err := memStore.ListEvents(context.Background(), model.EventQuery{
			UserID:   fmt.Sprintf("user-%d", i),
			Page:     1,
			PageSize: 100,
		})
		if err != nil {
			t.Fatalf("Failed to list events for user-%d: %v", i, err)
		}
		if len(events) == 0 {
			lostEvents++
		}
	}

	memStore2 := store.NewMemoryStore(log)
	scheduler := service.NewScheduler(memStore2, log)
	scheduler.Start()
	scheduler.Stop()

	taskNames := scheduler.GetTaskNames()

	hasDefect := false
	if lostEvents > 0 {
		hasDefect = true
		fmt.Printf("RED (红灯，缺陷未修复)\n")
		fmt.Printf("  - 事件丢失: %d/%d 个事件在关闭时未能持久化\n", lostEvents, numEvents)
	}
	if len(taskNames) > 0 {
		hasDefect = true
		if lostEvents == 0 {
			fmt.Printf("RED (红灯，缺陷未修复)\n")
		}
		fmt.Printf("  - 调度器泄漏: 停止后仍有 %d 个任务未清理: %v\n", len(taskNames), taskNames)
	}

	if !hasDefect {
		fmt.Printf("GREEN (绿灯，缺陷已修复)\n")
	}

	if hasDefect {
		t.Fail()
	}
}
