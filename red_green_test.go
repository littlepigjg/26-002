package main

import (
	"context"
	"io"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func TestRedGreen(t *testing.T) {
	log := logger.New(io.Discard, logger.LevelInfo, "test")
	memStore := store.NewMemoryStore(log)

	ctx := context.Background()

	// Create scheduler and run cleanup task first
	scheduler := service.NewScheduler(memStore, log)
	scheduler.Start()

	// Wait a bit for scheduler's initial task execution to complete
	time.Sleep(10 * time.Millisecond)

	// Create 5 sessions with EndTime in the past (1 second ago)
	pastTime := time.Now().Add(-1 * time.Second)
	for i := 0; i < 5; i++ {
		sess := &model.Session{
			ID:         model.GenerateTestID(),
			UserID:     "user1",
			UserType:   model.UserNew,
			DeviceType: model.DeviceDesktop,
			State:      model.SessionActive,
			StartTime:  pastTime.Add(-1 * time.Second),
			EndTime:    pastTime,
			CreatedAt:  pastTime.Add(-1 * time.Second),
			UpdatedAt:  pastTime.Add(-1 * time.Second),
			Pages:      make([]string, 0),
		}
		if err := memStore.CreateSession(ctx, sess); err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}
	}

	// Verify active session count is 5
	count, err := memStore.ActiveSessionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get active session count: %v", err)
	}
	if count != 5 {
		t.Fatalf("Expected 5 active sessions, got %d", count)
	}

	// Manually trigger the expiry with current time
	cutoff := time.Now()
	expiredCount, err := memStore.ExpireSessions(ctx, cutoff)
	if err != nil {
		t.Fatalf("Failed to expire sessions: %v", err)
	}
	if expiredCount != 5 {
		t.Fatalf("Expected 5 expired sessions, got %d", expiredCount)
	}

	// Now check the active session count - it should be 0 after cleanup
	// But due to the bug, the activeSessions index is not cleaned up
	count, err = memStore.ActiveSessionCount(ctx)
	if err != nil {
		t.Fatalf("Failed to get active session count: %v", err)
	}

	if count == 0 {
		t.Log("GREEN（绿灯，缺陷已修复）")
	} else {
		t.Logf("RED（红灯，缺陷未修复）")
		t.Errorf("Active sessions index still has %d entries after all sessions expired", count)
	}

	// Also test the scheduler's cleanup path
	scheduler.Stop()

	// Test with more complex scenario - multiple expire/clean cycles
	t.Run("MultipleExpireCycles", func(t *testing.T) {
		memStore2 := store.NewMemoryStore(log)
		ctx2 := context.Background()

		// Create first batch of sessions
		for i := 0; i < 3; i++ {
			sess := model.NewSession("user2", model.DeviceMobile, 10*time.Millisecond)
			if err := memStore2.CreateSession(ctx2, sess); err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}
		}

		time.Sleep(50 * time.Millisecond)

		// Expire first batch
		memStore2.ExpireSessions(ctx2, time.Now())

		// Create second batch
		for i := 0; i < 3; i++ {
			sess := model.NewSession("user2", model.DeviceTablet, 10*time.Millisecond)
			if err := memStore2.CreateSession(ctx2, sess); err != nil {
				t.Fatalf("Failed to create session: %v", err)
			}
		}

		time.Sleep(50 * time.Millisecond)

		// Expire second batch
		memStore2.ExpireSessions(ctx2, time.Now())

		// Check if active sessions index accumulated expired entries
		count2, err := memStore2.ActiveSessionCount(ctx2)
		if err != nil {
			t.Fatalf("Failed to get active session count: %v", err)
		}

		if count2 == 0 {
			t.Log("GREEN（绿灯，缺陷已修复）")
		} else {
			t.Log("RED（红灯，缺陷未修复）")
			t.Errorf("Active sessions index accumulated %d expired entries over multiple cycles", count2)
		}
	})

	// Test raw snapshot shows the issue
	t.Run("RawSnapshotVerification", func(t *testing.T) {
		memStore3 := store.NewMemoryStore(log)
		ctx3 := context.Background()

		// Create a session
		sess := model.NewSession("user3", model.DeviceDesktop, 10*time.Millisecond)
		if err := memStore3.CreateSession(ctx3, sess); err != nil {
			t.Fatalf("Failed to create session: %v", err)
		}

		// Verify snapshot has the session
		snapshot := memStore3.RawSnapshot()
		if len(snapshot) != 1 {
			t.Fatalf("Expected 1 session in snapshot, got %d", len(snapshot))
		}

		time.Sleep(50 * time.Millisecond)

		// Expire the session
		memStore3.ExpireSessions(ctx3, time.Now())

		// Verify snapshot still shows the expired session (in sessions map)
		snapshot = memStore3.RawSnapshot()
		if len(snapshot) != 1 {
			t.Fatalf("Expected 1 session in snapshot, got %d", len(snapshot))
		}

		// The activeSessions index should have been cleaned
		// Check if ActiveSessionCount returns correct value
		count3, err := memStore3.ActiveSessionCount(ctx3)
		if err != nil {
			t.Fatalf("Failed to get active session count: %v", err)
		}

		if count3 == 0 {
			t.Log("GREEN（绿灯，缺陷已修复）")
		} else {
			t.Log("RED（红灯，缺陷未修复）")
			t.Errorf("Active session count is %d but should be 0 after session expired", count3)
		}
	})
}
