package bug4_test

import (
	"context"
	"fmt"
	"io"
	"math"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	cfg := config.DefaultConfig()
	cfg.Store.MaxPathLength = 1000
	log := logger.New(io.Discard, logger.LevelError, "")
	memStore := store.NewMemoryStore(log)
	pathSvc := service.NewPathService(memStore, cfg, log)

	const userID = "test-user-001"
	const sessionID = "test-session-001"
	now := time.Now()

	// Test 1: DurationMs int64 overflow in PathSequence.ComputeDuration
	t.Run("Test1_DurationOverflow", func(t *testing.T) {
		ps := model.NewPathSequence(userID, sessionID)
		halfMax := int64(math.MaxInt64/2) + 1
		for i := 0; i < 3; i++ {
			ps.AppendNode(model.PathNode{
				PageURL:   fmt.Sprintf("/page%d", i),
				PageTitle: fmt.Sprintf("Page %d", i),
				Order:     i,
				Timestamp: now.Add(time.Duration(i) * time.Second),
				Duration:  halfMax,
			})
		}
		duration := ps.ComputeDuration()
		if duration < 0 {
			t.Logf("RED: DurationMs overflow detected — ComputeDuration() returned negative: %d", duration)
		} else {
			t.Logf("GREEN: DurationMs properly handled, value: %d", duration)
		}
	})

	// Test 2: AccumulateDuration overflow
	t.Run("Test2_AccumulateOverflow", func(t *testing.T) {
		halfMax := int64(math.MaxInt64/2) + 1
		var total int64
		for i := 0; i < 3; i++ {
			total = model.AccumulateDuration(total, halfMax)
		}
		if total < 0 {
			t.Logf("RED: AccumulateDuration overflow — total became negative: %d", total)
		} else {
			t.Logf("GREEN: AccumulateDuration properly handled, total: %d", total)
		}
	})

	// Test 3: Context lifecycle in ComputePathSequence
	t.Run("Test3_ContextLifecycle", func(t *testing.T) {
		ctx := context.Background()
		session := &model.Session{
			ID:            sessionID,
			UserID:        userID,
			UserType:      model.UserNew,
			DeviceType:    model.DeviceDesktop,
			State:         model.SessionActive,
			StartTime:     now,
			EndTime:       now.Add(30 * time.Minute),
			LastEventTime: now,
			CreatedAt:     now,
			UpdatedAt:     now,
		}
		memStore.CreateSession(ctx, session)

		for i := 0; i < 3; i++ {
			event := &model.Event{
				ID:         fmt.Sprintf("evt-%d", i),
				UserID:     userID,
				SessionID:  sessionID,
				Type:       model.EventPageView,
				PageURL:    fmt.Sprintf("/page%d", i),
				PageTitle:  fmt.Sprintf("Page %d", i),
				DurationMs: 1000,
				DeviceType: model.DeviceDesktop,
				Timestamp:  now.Add(time.Duration(i) * time.Second),
				CreatedAt:  now.Add(time.Duration(i) * time.Second),
			}
			memStore.CreateEvent(ctx, event)
		}

		cancelCtx, cancel := context.WithCancel(context.Background())
		cancel()

		_, err := pathSvc.ComputePathSequence(cancelCtx, session)
		if err == nil {
			t.Log("RED: Context lifecycle violated — function completed despite cancelled context")
		} else {
			t.Logf("GREEN: Context properly respected, error: %v", err)
		}
	})

	// Test 4: Context lifecycle in GetPopularPages
	t.Run("Test4_GetPopularPagesContext", func(t *testing.T) {
		cancelledCtx, cancelFn := context.WithTimeout(context.Background(), 1*time.Nanosecond)
		defer cancelFn()
		time.Sleep(10 * time.Nanosecond)

		_, err := pathSvc.GetPopularPages(cancelledCtx, now.Add(-time.Hour), now.Add(time.Hour), 10)
		if err == nil {
			t.Log("RED: GetPopularPages ignored cancelled context")
		} else {
			t.Logf("GREEN: GetPopularPages respected context, error: %v", err)
		}
	})

	// Final RED/GREEN determination
	hasDefect := false

	// Re-check Test 1
	ps := model.NewPathSequence(userID, sessionID)
	halfMax := int64(math.MaxInt64/2) + 1
	for i := 0; i < 3; i++ {
		ps.AppendNode(model.PathNode{
			PageURL:   fmt.Sprintf("/page%d", i),
			PageTitle: fmt.Sprintf("Page %d", i),
			Order:     i,
			Timestamp: now.Add(time.Duration(i) * time.Second),
			Duration:  halfMax,
		})
	}
	duration := ps.ComputeDuration()
	if duration < 0 {
		t.Log("FINAL: RED — DurationMs int64 overflow not handled")
		hasDefect = true
	}

	// Re-check Test 2
	var total int64
	for i := 0; i < 3; i++ {
		total = model.AccumulateDuration(total, halfMax)
	}
	if total < 0 {
		t.Log("FINAL: RED — AccumulateDuration overflow not handled")
		hasDefect = true
	}

	// Re-check Test 3
	cancelCtx, cancel := context.WithCancel(context.Background())
	cancel()
	session3 := &model.Session{
		ID:            "session-3",
		UserID:        userID,
		UserType:      model.UserNew,
		DeviceType:    model.DeviceDesktop,
		State:         model.SessionActive,
		StartTime:     now,
		EndTime:       now.Add(30 * time.Minute),
		LastEventTime: now,
		CreatedAt:     now,
		UpdatedAt:     now,
	}
	memStore.CreateSession(context.Background(), session3)
	_, err3 := pathSvc.ComputePathSequence(cancelCtx, session3)
	if err3 == nil {
		t.Log("FINAL: RED — Context lifecycle violated in ComputePathSequence")
		hasDefect = true
	}

	// Re-check Test 4
	cancelledCtx, cancelFn := context.WithTimeout(context.Background(), 1*time.Nanosecond)
	defer cancelFn()
	time.Sleep(10 * time.Nanosecond)
	_, err4 := pathSvc.GetPopularPages(cancelledCtx, now.Add(-time.Hour), now.Add(time.Hour), 10)
	if err4 == nil {
		t.Log("FINAL: RED — Context lifecycle violated in GetPopularPages")
		hasDefect = true
	}

	if hasDefect {
		t.Log("RESULT: RED (红灯，缺陷未修复)")
		t.FailNow()
	} else {
		t.Log("RESULT: GREEN (绿灯，缺陷已修复)")
	}
}
