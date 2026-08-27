package main

import (
	"context"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	log := logger.New(nil, logger.LevelError, "test")
	memStore := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()

	es := service.NewEventService(memStore, cfg, log)
	defer es.Stop()

	var guardMu sync.Mutex
	var panicCount int32
	panicMessages := make([]string, 0)

	es.SetPanicGuard(func(code, rawURL string) bool {
		if rawURL == "/trigger-race" {
			guardMu.Lock()
			atomic.AddInt32(&panicCount, 1)
			panicMessages = append(panicMessages, "service-level panic guard triggered for corruption probe")
			guardMu.Unlock()
			return true
		}
		return false
	})

	memStore.SetPanicGuard(func(code, rawURL string) bool {
		if rawURL == "/trigger-race" {
			guardMu.Lock()
			atomic.AddInt32(&panicCount, 1)
			panicMessages = append(panicMessages, "store-level panic guard triggered for corruption probe")
			guardMu.Unlock()
			return true
		}
		return false
	})

	ctx := context.Background()
	var wg sync.WaitGroup
	numGoroutines := 50
	numEventsPerGoroutine := 20

	var mu sync.Mutex
	var allEventIDs []string
	userEventCounts := make(map[string]int)
	successCount := 0

	for i := 0; i < numGoroutines; i++ {
		wg.Add(1)
		go func(goroutineID int) {
			defer wg.Done()
			defer func() {
				if r := recover(); r != nil {
					guardMu.Lock()
					atomic.AddInt32(&panicCount, 1)
					panicMessages = append(panicMessages, fmt.Sprintf("recovered panic: %v", r))
					guardMu.Unlock()
				}
			}()
			for j := 0; j < numEventsPerGoroutine; j++ {
				pageURL := fmt.Sprintf("/page-%d-%d", goroutineID, j)
				if goroutineID == 0 && j == 0 {
					pageURL = "/trigger-race"
				}
				req := &model.EventCreateRequest{
					UserID:    fmt.Sprintf("user-%d", goroutineID%5),
					Type:      model.EventPageView,
					PageURL:   pageURL,
					Timestamp: time.Now(),
				}
				ev, err := es.CreateEvent(ctx, req)
				if err == nil && ev != nil {
					mu.Lock()
					allEventIDs = append(allEventIDs, ev.ID)
					userEventCounts[ev.UserID]++
					successCount++
					mu.Unlock()
				}
			}
		}(i)
	}

	for i := 0; i < 10; i++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for j := 0; j < 200; j++ {
				_, _, _ = memStore.ListEvents(ctx, model.EventQuery{})
				_ = memStore.RawSnapshot()
			}
		}()
	}

	wg.Wait()

	if panicCount > 0 {
		t.Logf("RED (红灯，缺陷未修复): 检测到 %d 次 panic", panicCount)
		for _, msg := range panicMessages {
			t.Logf("  panic: %s", msg)
		}
		t.FailNow()
		return
	}

	inconsistency := es.ConsistencyCheck()
	if inconsistency != "" {
		t.Logf("RED (红灯，缺陷未修复): 数据索引不一致 - %s", inconsistency)
		t.FailNow()
		return
	}

	snapshot := memStore.RawSnapshot()
	for uid, expected := range userEventCounts {
		userActual := memStore.UserEventCount(uid)
		if userActual != expected {
			t.Logf("RED (红灯，缺陷未修复): 用户 %s 的事件索引数量不匹配，期望 %d，实际 %d",
				uid, expected, userActual)
			t.Logf("  快照中事件总数: %d", len(snapshot))
			t.FailNow()
			return
		}
	}

	t.Logf("GREEN (绿灯，缺陷已修复): 并发请求处理正常，共成功完成 %d 次操作", successCount)
}
