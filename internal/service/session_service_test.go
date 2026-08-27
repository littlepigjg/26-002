package service

import (
	"context"
	"errors"
	"io"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func newSessionService(t *testing.T) (*SessionService, *store.MemoryStore, *config.Config) {
	t.Helper()
	log := logger.New(io.Discard, logger.LevelError, "test")
	st := store.NewMemoryStore(log)
	cfg := config.DefaultConfig()
	cfg.Session.TimeoutMinutes = 60 // 1h timeout window, matching the reported scenario
	return NewSessionService(st, cfg, log), st, cfg
}

// newPageViewEvent mirrors what a tracking ingest path hands to BuildSession.
func newPageViewEvent(userID, pageURL string, ts time.Time) *model.Event {
	return &model.Event{
		ID:        "evt-" + pageURL,
		UserID:    userID,
		Type:      model.EventPageView,
		PageURL:   pageURL,
		Timestamp: ts,
		DeviceType: model.DeviceDesktop,
	}
}

// TestBuildSession_DoesNotReviveExpiredSession is the end-to-end repro of the
// reported bug. After ExpireSessions closes the session, a subsequent event
// for the same user must start a NEW session rather than be attributed to the
// expired one. Before the fix, the expired session was silently reused and
// the event was attributed to a session that should have been closed.
func TestBuildSession_DoesNotReviveExpiredSession(t *testing.T) {
	ctx := context.Background()
	svc, st, _ := newSessionService(t)

	base := time.Now()

	// 1. Create a session via BuildSession (first event).
	first, err := svc.BuildSession(ctx, newPageViewEvent("user-A", "/home", base))
	if err != nil {
		t.Fatalf("BuildSession (1st): %v", err)
	}
	if first.State != model.SessionActive {
		t.Fatalf("expected active, got %q", first.State)
	}

	// 2. Force-expire it as of 2h in the future (timeout window is 1h).
	if _, err := svc.ExpireSessions(ctx, base.Add(2*time.Hour)); err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}

	// Confirm the stored session is now expired.
	stored, err := st.GetSession(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if stored.State != model.SessionExpired {
		t.Fatalf("expected %q, got %q", model.SessionExpired, stored.State)
	}

	// 3. A second event arrives well within the timeout of the previous
	//    event's timestamp (so the OLD code would reuse the session). The
	//    fix must create a brand-new session instead.
	second, err := svc.BuildSession(ctx, newPageViewEvent("user-A", "/pricing", base.Add(1*time.Minute)))
	if err != nil {
		t.Fatalf("BuildSession (2nd): %v", err)
	}
	if second.ID == first.ID {
		t.Fatalf("expected a new session after expiry, but reused %s", first.ID)
	}
	if second.State != model.SessionActive {
		t.Fatalf("expected new session active, got %q", second.State)
	}

	// The expired session must remain expired and untouched.
	again, err := st.GetSession(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetSession (again): %v", err)
	}
	if again.State != model.SessionExpired {
		t.Fatalf("expired session state mutated to %q", again.State)
	}
	if len(again.Pages) != 1 || again.Pages[0] != "/home" {
		t.Fatalf("expired session pages mutated: %v", again.Pages)
	}
}

// TestProcessExistingEvent_RejectsExpiredSession directly exercises the
// service-layer guard added to processExistingEvent: feeding an event into an
// already-expired session must yield ErrInvalidState.
func TestProcessExistingEvent_RejectsExpiredSession(t *testing.T) {
	ctx := context.Background()
	svc, st, cfg := newSessionService(t)

	base := time.Now()
	first, err := svc.BuildSession(ctx, newPageViewEvent("user-B", "/home", base))
	if err != nil {
		t.Fatalf("BuildSession: %v", err)
	}
	if _, err := svc.ExpireSessions(ctx, base.Add(2*time.Hour)); err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}

	stored, err := st.GetSession(ctx, first.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	err = svc.processExistingEvent(stored, newPageViewEvent("user-B", "/x", base.Add(1*time.Minute)), cfg.Session.Timeout())
	if !errors.Is(err, model.ErrInvalidState) {
		t.Fatalf("expected ErrInvalidState from processExistingEvent on expired session, got %v", err)
	}
}
