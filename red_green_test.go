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

// TestRedGreen tests the BUG-013: event buffer processing panic not recovered
func TestRedGreen(t *testing.T) {
	log := logger.New(os.Stderr, logger.LevelError, "")
	cfg := config.DefaultConfig()
	cfg.Store.FlushInterval = 1 // Set short flush interval (1 second)

	memStore := store.NewMemoryStore(log)
	es := service.NewEventService(memStore, cfg, log)

	defer func() {
		memStore.Close()
		es.Stop()
	}()

	t.Run("RED_GREEN", func(t *testing.T) {
		// Step 1: Verify EventService is healthy initially
		if !es.IsHealthy() {
			t.Fatal("EventService should be healthy initially")
		}
		t.Log("Initial health check: PASS")

		// Step 2: Create a normal event
		ctx := context.Background()
		req1 := &model.EventCreateRequest{
			UserID:      "user_1",
			Type:        model.EventPageView,
			PageURL:     "https://example.com/page1",
			DeviceType:  model.DeviceDesktop,
		}

		_, err := es.CreateEvent(ctx, req1)
		if err != nil {
			t.Fatalf("Failed to create event: %v", err)
		}
		t.Log("First event created successfully")

		// Step 3: Set panic guard that triggers panic for user_2
		es.SetPanicGuard(func(userID, pageURL string) bool {
			return userID == "user_2"
		})
		t.Log("Panic guard set for user_2")

		// Step 4: Create an event that will trigger panic in background goroutine
		req2 := &model.EventCreateRequest{
			UserID:      "user_2",
			Type:        model.EventPageView,
			PageURL:     "https://example.com/page2",
			DeviceType:  model.DeviceDesktop,
		}

		_, err = es.CreateEvent(ctx, req2)
		if err != nil {
			t.Logf("CreateEvent returned error: %v", err)
		}
		t.Log("Second event created (will trigger panic in background)")

		// Step 5: Wait for the flush to be triggered by ticker
		// FlushInterval is 1 second, so we need to wait at least 1 second
		// But we should also account for the time it takes to process
		t.Log("Waiting for flush buffer to be triggered...")
		time.Sleep(1200 * time.Millisecond) // Wait for flush (1s interval + buffer time)

		// Step 6: Check if EventService is healthy after the panic
		// If the bug exists, recovered will be false (set to false before processing,
		// but never set back to true after panic recovery)
		isHealthy := es.IsHealthy()
		t.Logf("Health check after flush: %v", isHealthy)

		if isHealthy {
			// GREEN: Bug is fixed
			// The recovered state was properly updated after panic recovery
			fmt.Println("GREEN（绿灯，缺陷已修复）")
			t.Log("Service is healthy after panic recovery - BUG FIXED")
		} else {
			// RED: Bug exists
			// The recovered state was not updated after panic recovery
			fmt.Println("RED（红灯，缺陷未修复）")
			t.Error("RED: Panic recovery state is not correctly updated")
			t.Error("The recover logic in flushBuffer does not set recovered = true")
			t.Error("This means the service appears unhealthy after a panic is recovered")
			t.Log("BUG: In event_service.go flushBuffer(), the recover block doesn't set es.recovered = true")
			t.Log("BUG: Also, the success path after processing events doesn't set es.recovered = true")
		}
	})
}

func TestMain(m *testing.M) {
	fmt.Println("Starting BUG-013 test: event buffer processing panic recovery")
	fmt.Println("This test verifies that panics in background goroutines are properly recovered")
	fmt.Println("and the service health state is correctly updated")
	m.Run()
}
