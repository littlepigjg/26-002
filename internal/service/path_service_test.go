package service

import (
	"context"
	"errors"
	"io"
	"math"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// newTestService wires a PathService against a real in-memory store with the
// default config and a quiet logger so tests exercise the production paths.
func newTestService(t *testing.T) (*PathService, *store.MemoryStore) {
	t.Helper()
	log := logger.New(io.Discard, logger.LevelError, "")
	st := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()
	return NewPathService(st, cfg, log), st
}

// TestComputePathSequence_CancelledContextReturnsError reproduces the reported
// bug: a caller passing an already-cancelled context used to get a successful
// path back instead of the context error. It must now return the context error
// and persist no path.
func TestComputePathSequence_CancelledContextReturnsError(t *testing.T) {
	ps, st := newTestService(t)

	session := model.NewSession("user-1", model.DeviceDesktop, 30*time.Minute)
	if err := st.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Seed a couple of page-view events so there is real work to (not) process.
	for i, page := range []string{"/home", "/products"} {
		ev := model.NewEvent(session.UserID, session.ID, model.EventPageView, page)
		ev.DurationMs = int64(10 * (i + 1))
		ev.Timestamp = time.Unix(int64(i), 0)
		if err := st.CreateEvent(context.Background(), ev); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
	}

	ctx, cancel := context.WithCancel(context.Background())
	cancel() // pre-cancel before the call

	path, err := ps.ComputePathSequence(ctx, session)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got err=%v path=%v", err, path)
	}
	if path != nil {
		t.Fatalf("expected nil path on cancelled context, got %+v", path)
	}

	// No path should have been persisted for the user.
	paths, err := ps.GetUserPaths(context.Background(), session.UserID)
	if err != nil {
		t.Fatalf("GetUserPaths: %v", err)
	}
	if len(paths) != 0 {
		t.Fatalf("expected no persisted paths, got %d", len(paths))
	}
}

// TestComputePathSequence_OverflowSaturates is the end-to-end repro: three
// page-view events with DurationMs = math.MaxInt64/2 + 1 each must yield a
// non-negative TotalDuration, saturating to math.MaxInt64 rather than the
// previously observed -4611686018427387905.
func TestComputePathSequence_OverflowSaturates(t *testing.T) {
	ps, st := newTestService(t)

	session := model.NewSession("user-1", model.DeviceDesktop, 30*time.Minute)
	if err := st.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	big := int64(math.MaxInt64/2 + 1)
	for i := 0; i < 3; i++ {
		ev := model.NewEvent(session.UserID, session.ID, model.EventPageView, "/page")
		ev.DurationMs = big
		ev.Timestamp = time.Unix(int64(i), 0)
		if err := st.CreateEvent(context.Background(), ev); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
	}

	path, err := ps.ComputePathSequence(context.Background(), session)
	if err != nil {
		t.Fatalf("ComputePathSequence: %v", err)
	}

	if path.TotalDuration < 0 {
		t.Fatalf("TotalDuration went negative: %d", path.TotalDuration)
	}
	if path.TotalDuration != math.MaxInt64 {
		t.Fatalf("TotalDuration = %d; want saturated math.MaxInt64 (%d)",
			path.TotalDuration, int64(math.MaxInt64))
	}
}

// TestComputePathSequence_NormalAccumulation confirms the happy path still sums
// ordinary durations correctly after the saturating rewrite.
func TestComputePathSequence_NormalAccumulation(t *testing.T) {
	ps, st := newTestService(t)

	session := model.NewSession("user-1", model.DeviceDesktop, 30*time.Minute)
	if err := st.CreateSession(context.Background(), session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	durations := []int64{100, 200, 300}
	var want int64
	for i, d := range durations {
		ev := model.NewEvent(session.UserID, session.ID, model.EventPageView, "/page")
		ev.DurationMs = d
		ev.Timestamp = time.Unix(int64(i), 0)
		want += d
		if err := st.CreateEvent(context.Background(), ev); err != nil {
			t.Fatalf("CreateEvent: %v", err)
		}
	}

	path, err := ps.ComputePathSequence(context.Background(), session)
	if err != nil {
		t.Fatalf("ComputePathSequence: %v", err)
	}
	if path.TotalDuration != want {
		t.Fatalf("TotalDuration = %d; want %d", path.TotalDuration, want)
	}
}
