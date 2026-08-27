package store

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

func newTestStore(t *testing.T) *MemoryStore {
	t.Helper()
	return NewMemoryStore(logger.New(io.Discard, logger.LevelError, "test"))
}

// newActiveSession creates an active session with a 1h timeout window.
func newActiveSession(userID string) *model.Session {
	return model.NewSession(userID, model.DeviceDesktop, time.Hour)
}

// addPageView mutates s as if processing a page-view event: bumps version,
// advances LastEventTime/EndTime and appends a page. It mirrors what
// SessionService.processExistingEvent -> Session.AddEvent does.
func addPageView(s *model.Session, pageURL string, ts time.Time) {
	s.LastEventTime = ts
	s.EventCount++
	s.EndTime = ts.Add(time.Hour)
	s.UpdatedAt = time.Now()
	s.IncrementVersion()
	s.Pages = append(s.Pages, pageURL)
}

// TestExpireThenUpdateRejected reproduces the reported regression: a session
// that has been expired must not accept a subsequent event-bearing update.
// Before the fix, UpdateSession succeeded because the expired branch of
// the state switch was empty and the version check was bypassed by the
// version bump from AddEvent.
func TestExpireThenUpdateRejected(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// 1. Create a session with a 1h timeout, stored as active.
	session := newActiveSession("user-1")
	session.LastEventTime = session.StartTime
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// 2. Force-expire it as of 2h in the future (end time is start+1h).
	before := session.StartTime.Add(2 * time.Hour)
	n, err := store.ExpireSessions(ctx, before)
	if err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}
	if n != 1 {
		t.Fatalf("expected 1 session expired, got %d", n)
	}

	// 3. Read it back; it must be expired.
	stored, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.State != model.SessionExpired {
		t.Fatalf("expected state %q, got %q", model.SessionExpired, stored.State)
	}

	// 4. Simulate "read back, add a new page-view event, write back".
	addPageView(stored, "/pricing", stored.StartTime.Add(1*time.Minute))

	// 5. Update MUST now be rejected — the session is terminal.
	err = store.UpdateSession(ctx, stored)
	if !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState when updating an expired session with new activity, got %v", err)
	}
}

// TestActiveSessionUpdateAccepted ensures the legitimate path — adding an
// event to a still-active session — still succeeds after the fix.
func TestActiveSessionUpdateAccepted(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	session := newActiveSession("user-2")
	// Simulate the service's create path: a freshly created session has
	// recorded its first event.
	session.LastEventTime = session.StartTime
	session.EventCount = 1
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Read back and add a second page-view event (mirrors BuildSession's
	// processExistingEvent -> AddEvent path on an active session).
	stored, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	addPageView(stored, "/about", stored.StartTime.Add(1*time.Minute))

	if err := store.UpdateSession(ctx, stored); err != nil {
		t.Fatalf("UpdateSession on active session: %v", err)
	}

	// EventCount must reflect the appended event.
	again, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession after update: %v", err)
	}
	if again.EventCount != 2 {
		t.Fatalf("expected event count 2, got %d", again.EventCount)
	}
	if again.State != model.SessionActive {
		t.Fatalf("expected state %q, got %q", model.SessionActive, again.State)
	}
}

// TestMetadataUpdateOnActiveAllowed ensures non-event metadata updates
// (e.g. ReclassifyUserType touching only UserType) on an active session
// still work even when the version is unchanged.
func TestMetadataUpdateOnActiveAllowed(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	session := newActiveSession("user-3")
	session.LastEventTime = session.StartTime
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	stored, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	// Pure metadata change: no LastEventTime/EventCount change, no version
	// bump — exactly what ReclassifyUserType does.
	stored.UserType = model.UserReturning

	if err := store.UpdateSession(ctx, stored); err != nil {
		t.Fatalf("metadata UpdateSession on active session: %v", err)
	}
}

// TestExpiredSessionStateFlipRejected ensures a terminal session cannot be
// flipped back to active without any new activity either.
func TestExpiredSessionStateFlipRejected(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	session := newActiveSession("user-4")
	session.LastEventTime = session.StartTime
	if err := store.CreateSession(ctx, session); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	before := session.StartTime.Add(2 * time.Hour)
	if _, err := store.ExpireSessions(ctx, before); err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}

	stored, err := store.GetSession(ctx, session.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	// Try to revive by simply setting State back to active with no new event.
	stored.State = model.SessionActive

	err = store.UpdateSession(ctx, stored)
	if !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState when reviving an expired session, got %v", err)
	}
}
