package service

import (
	"context"
	"io"
	"strconv"
	"testing"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func newTestLogger(t *testing.T) *logger.Logger {
	t.Helper()
	return logger.New(io.Discard, logger.LevelDebug, "test")
}

func newTestEventService(t *testing.T) (*EventService, *store.MemoryStore) {
	t.Helper()
	log := newTestLogger(t)
	st := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()
	// A long flush interval ensures the ticker never fires during the test,
	// so persistence only happens via the explicit Stop() final flush.
	cfg.Store.FlushInterval = 3600
	return NewEventService(st, cfg, log), st
}

func pageViewReq(userID, pageURL string) *model.EventCreateRequest {
	return &model.EventCreateRequest{
		UserID:   userID,
		Type:     model.EventPageView,
		PageURL:  pageURL,
	}
}

// TestEventService_StopFlushesBufferedEvents reproduces the reported data-loss
// regression: events created programmatically and immediately followed by a
// graceful Stop() must all be persisted. Before the fix, Stop() closed stopCh
// before calling flushBuffer(), and flushBuffer() short-circuited on a closed
// stopCh, dropping every buffered event.
func TestEventService_StopFlushesBufferedEvents(t *testing.T) {
	es, st := newTestEventService(t)
	defer st.Close()

	const n = 50
	ctx := context.Background()
	for i := 0; i < n; i++ {
		req := pageViewReq(
			"user-"+strconv.Itoa(i),
			"https://example.com/page/"+strconv.Itoa(i),
		)
		if _, err := es.CreateEvent(ctx, req); err != nil {
			t.Fatalf("CreateEvent %d failed: %v", i, err)
		}
	}

	// Graceful shutdown immediately after ingestion — no sleep.
	es.Stop()

	events, total, err := st.ListEvents(ctx, model.EventQuery{Page: 1, PageSize: n * 2})
	if err != nil {
		t.Fatalf("ListEvents failed: %v", err)
	}
	if total != n {
		t.Fatalf("expected %d persisted events, got %d (data was lost on shutdown)", n, total)
	}
	if len(events) != n {
		t.Fatalf("expected %d event rows, got %d", n, len(events))
	}
}

// TestEventService_StopFlushesAfterPriorFlush verifies the removed `flushed`
// flag did not make the final flush a no-op: ingest enough events to trigger a
// batch flush (>= 100), then add more, then Stop() must still persist the
// second batch.
func TestEventService_StopFlushesAfterPriorFlush(t *testing.T) {
	es, st := newTestEventService(t)
	defer st.Close()

	ctx := context.Background()
	// First 100 events trigger the immediate batch flush in CreateEvent.
	for i := 0; i < 100; i++ {
		if _, err := es.CreateEvent(ctx, pageViewReq("u-"+strconv.Itoa(i), "/p/"+strconv.Itoa(i))); err != nil {
			t.Fatalf("CreateEvent %d: %v", i, err)
		}
	}
	// Additional buffered events rely on the final flush.
	for i := 100; i < 130; i++ {
		if _, err := es.CreateEvent(ctx, pageViewReq("u-"+strconv.Itoa(i), "/p/"+strconv.Itoa(i))); err != nil {
			t.Fatalf("CreateEvent %d: %v", i, err)
		}
	}

	es.Stop()

	_, total, err := st.ListEvents(ctx, model.EventQuery{Page: 1, PageSize: 500})
	if err != nil {
		t.Fatalf("ListEvents: %v", err)
	}
	if total != 130 {
		t.Fatalf("expected 130 persisted events, got %d", total)
	}
}

// TestEventService_StopIdempotent ensures calling Stop twice does not panic
// (e.g. closing stopCh twice).
func TestEventService_StopIdempotent(t *testing.T) {
	es, st := newTestEventService(t)
	defer st.Close()

	es.Stop()
	es.Stop() // must not panic
}
