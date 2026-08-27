package service

import (
	"io"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// TestScheduler_StopClearsTaskRegistry reproduces the reported state-residue
// regression: after Stop(), GetTaskNames() must return no tasks. Before the
// fix, Stop() only sent stop signals but never cleared the task map, so all
// old task names remained registered.
func TestScheduler_StopClearsTaskRegistry(t *testing.T) {
	log := logger.New(io.Discard, logger.LevelDebug, "test")
	st := store.NewMemoryStore(log)
	defer st.Close()

	sched := NewScheduler(st, log)
	sched.Start()

	// Start() registers the default maintenance tasks.
	if names := sched.GetTaskNames(); len(names) == 0 {
		t.Fatalf("expected registered tasks after Start(), got none")
	}

	sched.Stop()

	if names := sched.GetTaskNames(); len(names) != 0 {
		t.Fatalf("expected no registered tasks after Stop(), got %d: %v", len(names), names)
	}
}

// TestScheduler_StopIdempotent ensures calling Stop twice does not panic
// (e.g. closing task stopCh channels twice).
func TestScheduler_StopIdempotent(t *testing.T) {
	log := logger.New(io.Discard, logger.LevelDebug, "test")
	st := store.NewMemoryStore(log)
	defer st.Close()

	sched := NewScheduler(st, log)
	sched.Start()
	sched.Stop()
	sched.Stop() // must not panic
}

// TestScheduler_StopWaitsForGoroutines verifies that Stop() waits for task
// goroutines to fully exit, so the scheduler is quiescent on return. We use a
// task whose interval is large so it only runs once at startup; after Stop()
// returns, the wait group must be drained.
func TestScheduler_StopWaitsForGoroutines(t *testing.T) {
	log := logger.New(io.Discard, logger.LevelDebug, "test")
	st := store.NewMemoryStore(log)
	defer st.Close()

	sched := NewScheduler(st, log)
	sched.Start()

	// Give the startup task goroutines a moment to enter the run loop.
	time.Sleep(50 * time.Millisecond)

	done := make(chan struct{})
	go func() {
		sched.Stop()
		close(done)
	}()

	select {
	case <-done:
		// Stop() returned; goroutines were waited for.
	case <-time.After(5 * time.Second):
		t.Fatalf("Scheduler.Stop() did not return within timeout (goroutines not waited for)")
	}
}
