package service

import (
	"context"
	"fmt"
	"sync"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// TestConcurrentCreateEventsRace reproduces the production load-test scenario:
// many writers calling CreateEvent concurrently while readers call ListEvents
// and RawSnapshot. Before the fix, this triggered
// "fatal error: concurrent map read and map write" because the store accessed
// the eventsByUser / eventsBySession indexes outside the lock, and lost
// updates caused the per-user index to drift out of sync with the events map.
//
// Run with -race; the test additionally asserts data consistency so the index
// drift is caught even if the runtime's concurrent-map detector does not fire.
func TestConcurrentCreateEventsRace(t *testing.T) {
	if !raceBuild {
		t.Skip("run with -race for the definitive concurrency check; skipped otherwise")
	}

	log := logger.New(discard{}, logger.LevelError, "")
	st := store.NewMemoryStore(log)
	defer st.Close()

	// Store-level guard: triggers a panic for the very first event of the
	// first goroutine (URL /trigger-race). The store must abort that single
	// event cleanly without corrupting shared state.
	var guardOnce sync.Once
	var triggered bool
	st.SetPanicGuard(func(code, rawURL string) bool {
		if rawURL == "/trigger-race" {
			doTrigger := false
			guardOnce.Do(func() { doTrigger = true; triggered = true })
			return doTrigger
		}
		return false
	})

	cfg := config.DefaultConfig()
	svc := NewEventService(st, cfg, log)

	// Service-level guard: no-op here, but must be exercised to prove the
	// production wiring path works under concurrency.
	svc.SetPanicGuard(func(code, rawURL string) bool { return false })

	const writers = 50
	const perWriter = 20
	const readers = 10

	var wg sync.WaitGroup
	// Writers
	for g := 0; g < writers; g++ {
		wg.Add(1)
		go func(gid int) {
			defer wg.Done()
			for i := 0; i < perWriter; i++ {
				req := &model.EventCreateRequest{
					UserID:     fmt.Sprintf("user-%d", gid%5),
					Type:       model.EventPageView,
					PageURL:    "/page",
					DeviceType: model.DeviceDesktop,
				}
				if gid == 0 && i == 0 {
					req.PageURL = "/trigger-race"
				}
				// CreateEvent panics when the store-level guard fires; that is
				// expected for exactly one event and must not tear down the
				// process. Recover so the writer goroutine records the rest.
				func() {
					defer func() { _ = recover() }()
					_, _ = svc.CreateEvent(context.Background(), req)
				}()
			}
		}(g)
	}
	// Readers hammering ListEvents + RawSnapshot concurrently with writers.
	for r := 0; r < readers; r++ {
		wg.Add(1)
		go func() {
			defer wg.Done()
			deadline := time.Now().Add(2 * time.Second)
			for time.Now().Before(deadline) {
				_, _, _ = svc.ListEvents(context.Background(), model.EventQuery{PageSize: 50})
				_ = st.RawSnapshot()
				_ = st.RawUserIndexSnapshot()
			}
		}()
	}
	wg.Wait()

	// Drain the service buffer so every buffered event lands in the store.
	svc.Stop()

	// Exactly one event should have been aborted by the panic guard.
	snapshot := st.RawSnapshot()
	if len(snapshot) != writers*perWriter-1 {
		t.Fatalf("expected %d events in store, got %d", writers*perWriter-1, len(snapshot))
	}

	if !triggered {
		t.Fatal("store-level panic guard never fired; test did not exercise the intended path")
	}

	if msg := svc.ConsistencyCheck(); msg != "" {
		t.Fatalf("consistency check failed: %s", msg)
	}

	// The per-user index total must equal the events map size. After the fix
	// there are no lost updates, so the two cannot drift apart.
	uid := st.RawUserIndexSnapshot()
	total := 0
	for _, ids := range uid {
		total += len(ids)
	}
	if total != len(snapshot) {
		t.Fatalf("index total %d != events map size %d", total, len(snapshot))
	}
}
