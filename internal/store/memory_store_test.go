package store

import (
	"context"
	"errors"
	"io"
	"testing"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

// newTestLogger returns a logger that writes to io.Discard, safe for tests.
func newTestLogger() *logger.Logger {
	return logger.New(io.Discard, logger.LevelDebug, "")
}

// TestCreateEventsContextCancelRollback reproduces the reported bug: when a
// context cancellation guard fires mid-batch, no event should be persisted.
func TestCreateEventsContextCancelRollback(t *testing.T) {
	store := NewMemoryStore(newTestLogger())
	store.SetContextCancelGuard(func(index int) bool {
		return index >= 2
	})

	events := []*model.Event{
		model.NewEvent("user-1", "ses-1", model.EventPageView, "https://example.com/a"),
		model.NewEvent("user-1", "ses-1", model.EventPageView, "https://example.com/b"),
		model.NewEvent("user-1", "ses-1", model.EventPageView, "https://example.com/c"),
	}
	ids := []string{events[0].ID, events[1].ID, events[2].ID}

	err := store.CreateEvents(context.Background(), events)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("expected context.Canceled, got %v", err)
	}

	// None of the events should have been persisted after the rollback.
	for i, id := range ids {
		if _, getErr := store.GetEvent(context.Background(), id); getErr == nil {
			t.Errorf("event at index %d (id=%s) should not have been stored after cancellation", i, id)
		}
	}
}

// TestCreateEventsSuccessPersistsAll ensures the happy path still stores every
// event, including session-indexed lookups.
func TestCreateEventsSuccessPersistsAll(t *testing.T) {
	store := NewMemoryStore(newTestLogger())

	events := []*model.Event{
		model.NewEvent("user-2", "ses-2", model.EventPageView, "https://example.com/a"),
		model.NewEvent("user-2", "ses-2", model.EventClick, "https://example.com/b"),
	}

	if err := store.CreateEvents(context.Background(), events); err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	for i, e := range events {
		if got, err := store.GetEvent(context.Background(), e.ID); err != nil || got == nil {
			t.Errorf("event at index %d should have been stored", i)
		}
	}
}
