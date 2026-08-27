package store

import (
	"context"
	"testing"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

func newTestStore(t *testing.T) *MemoryStore {
	t.Helper()
	return NewMemoryStore(logger.New(nil, logger.LevelDebug, "test"))
}

func makeEndedSession(userID string, endTime time.Time) *model.Session {
	s := model.NewSession(userID, model.DeviceDesktop, 30*time.Minute)
	s.EndTime = endTime
	s.StartTime = endTime.Add(-2 * time.Hour)
	s.LastEventTime = endTime
	return s
}

// TestExpireSessionsCleansActiveIndex reproduces the reported bug: after
// expiring sessions that have already ended, ActiveSessionCount must reflect
// the real active state (0), not the pre-expire count.
func TestExpireSessionsCleansActiveIndex(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// Create 5 sessions that already ended in the past.
	pastEnd := time.Now().Add(-1 * time.Hour)
	for i := 0; i < 5; i++ {
		s := makeEndedSession("u1", pastEnd)
		if err := store.CreateSession(ctx, s); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}

	// Sanity: all 5 are tracked as active before expiry.
	if got, _ := store.ActiveSessionCount(ctx); got != 5 {
		t.Fatalf("active count before expire = %d, want 5", got)
	}

	// Expire sessions whose EndTime is before now.
	count, err := store.ExpireSessions(ctx, time.Now())
	if err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}
	if count != 5 {
		t.Fatalf("expired count = %d, want 5", count)
	}

	// The active index must now be empty.
	got, _ := store.ActiveSessionCount(ctx)
	if got != 0 {
		t.Fatalf("active count after expire = %d, want 0", got)
	}
}

// TestExpireSessionsPartialLeave verifies that not-yet-expired sessions are
// left in the active index while older ones are removed.
func TestExpireSessionsPartialLeave(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	futureEnd := time.Now().Add(1 * time.Hour)
	pastEnd := time.Now().Add(-1 * time.Hour)

	// Two already-ended sessions and one still-valid session.
	for i := 0; i < 2; i++ {
		if err := store.CreateSession(ctx, makeEndedSession("u1", pastEnd)); err != nil {
			t.Fatalf("CreateSession: %v", err)
		}
	}
	live := makeEndedSession("u2", futureEnd)
	if err := store.CreateSession(ctx, live); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	if _, err := store.ExpireSessions(ctx, time.Now()); err != nil {
		t.Fatalf("ExpireSessions: %v", err)
	}

	got, _ := store.ActiveSessionCount(ctx)
	if got != 1 {
		t.Fatalf("active count after partial expire = %d, want 1", got)
	}

	// The surviving session must be the live one.
	s, err := store.GetSession(ctx, live.ID)
	if err != nil {
		t.Fatalf("GetSession: %v", err)
	}
	if s.State != model.SessionActive {
		t.Fatalf("surviving session state = %s, want active", s.State)
	}
}

// TestCleanupExpiredSessionsRespectsBefore confirms CleanupExpiredSessions
// honors the passed cutoff instead of always comparing against time.Now().
func TestCleanupExpiredSessionsRespectsBefore(t *testing.T) {
	ctx := context.Background()
	store := newTestStore(t)

	// A session that ended 2 hours ago, already marked expired.
	old := makeEndedSession("u1", time.Now().Add(-2*time.Hour))
	old.State = model.SessionExpired
	if err := store.CreateSession(ctx, old); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// A session that ended 10 minutes ago, also marked expired.
	recent := makeEndedSession("u1", time.Now().Add(-10*time.Minute))
	recent.State = model.SessionExpired
	if err := store.CreateSession(ctx, recent); err != nil {
		t.Fatalf("CreateSession: %v", err)
	}

	// Cutoff 30 minutes ago: only the 2h-old session should be cleaned.
	cutoff := time.Now().Add(-30 * time.Minute)
	count, err := store.CleanupExpiredSessions(ctx, cutoff)
	if err != nil {
		t.Fatalf("CleanupExpiredSessions: %v", err)
	}
	if count != 1 {
		t.Fatalf("cleanup count = %d, want 1", count)
	}

	got, _ := store.ActiveSessionCount(ctx)
	if got != 1 {
		t.Fatalf("active count after cleanup = %d, want 1 (recent expired retained)", got)
	}
}
