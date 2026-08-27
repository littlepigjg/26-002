package store

import (
	"context"
	"testing"

	"github.com/ubaas/ubaas/internal/model"
)

// newTestUserStore builds a UserStore backed by a fresh in-memory store.
func newTestUserStore(t *testing.T) *UserStore {
	t.Helper()
	ms := NewMemoryStore(nil)
	return NewUserStore(ms)
}

// TestUpdateUser_PreservesDimensionsAcrossSparseEvent reproduces the reported
// production bug: an anonymous user first reports a page_view carrying full
// dimension info (device_type, os, browser, country) and then reports a click
// event that omits all dimension fields. The second event must not erase the
// values learned from the first.
func TestUpdateUser_PreservesDimensionsAcrossSparseEvent(t *testing.T) {
	us := newTestUserStore(t)
	ctx := context.Background()
	anonID := "anonymous-user"

	// First event: rich page_view with all dimensions.
	rich := &model.Event{
		UserID:     anonID,
		Type:       model.EventPageView,
		PageURL:    "https://example.com/home",
		DeviceType: model.DeviceMobile,
		OS:         "Android",
		Browser:    "Chrome",
		Country:    "US",
	}
	if _, err := us.UpdateUser(ctx, rich); err != nil {
		t.Fatalf("UpdateUser (rich): %v", err)
	}

	ud, err := us.GetUser(ctx, anonID)
	if err != nil {
		t.Fatalf("GetUser after rich event: %v", err)
	}
	if ud.DeviceType != model.DeviceMobile || ud.OS != "Android" || ud.Browser != "Chrome" || ud.Country != "US" {
		t.Fatalf("dimensions not stored after rich event: %+v", ud)
	}

	// Second event: a sparse click that only carries type and page_url.
	sparse := &model.Event{
		UserID:  anonID,
		Type:    model.EventClick,
		PageURL: "https://example.com/home",
		// DeviceType, OS, Browser, Country intentionally omitted (zero values).
	}
	if _, err := us.UpdateUser(ctx, sparse); err != nil {
		t.Fatalf("UpdateUser (sparse): %v", err)
	}

	ud2, err := us.GetUser(ctx, anonID)
	if err != nil {
		t.Fatalf("GetUser after sparse event: %v", err)
	}

	if ud2.DeviceType != model.DeviceMobile {
		t.Errorf("DeviceType = %q, want %q (should be preserved from first event)", ud2.DeviceType, model.DeviceMobile)
	}
	if ud2.OS != "Android" {
		t.Errorf("OS = %q, want %q (should be preserved from first event)", ud2.OS, "Android")
	}
	if ud2.Browser != "Chrome" {
		t.Errorf("Browser = %q, want %q (should be preserved from first event)", ud2.Browser, "Chrome")
	}
	if ud2.Country != "US" {
		t.Errorf("Country = %q, want %q (should be preserved from first event)", ud2.Country, "US")
	}

	// Counters and timestamps should still advance.
	if ud2.EventCount != 2 {
		t.Errorf("EventCount = %d, want 2", ud2.EventCount)
	}
	if ud2.SessionCount != 1 {
		t.Errorf("SessionCount = %d, want 1 (one page_view)", ud2.SessionCount)
	}
}

// TestUpdateUser_NewDimensionWhenUnknown verifies a first-time user still
// gets dimensions recorded when the first event carries them.
func TestUpdateUser_NewDimensionWhenUnknown(t *testing.T) {
	us := newTestUserStore(t)
	ctx := context.Background()

	if _, err := us.UpdateUser(ctx, &model.Event{
		UserID:     "u1",
		Type:       model.EventPageView,
		PageURL:    "https://example.com/",
		DeviceType: model.DeviceDesktop,
		OS:         "macOS",
		Browser:    "Firefox",
		Country:    "GB",
	}); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	ud, err := us.GetUser(ctx, "u1")
	if err != nil {
		t.Fatalf("GetUser: %v", err)
	}
	if ud.DeviceType != model.DeviceDesktop || ud.OS != "macOS" || ud.Browser != "Firefox" || ud.Country != "GB" {
		t.Fatalf("dimensions not stored for first-time user: %+v", ud)
	}
}

// TestUpdateUser_AnonymousUserFallback verifies that an event with no user_id
// is tracked under the anonymous-user identifier, matching the production
// behavior described in the bug report.
func TestUpdateUser_AnonymousUserFallback(t *testing.T) {
	us := newTestUserStore(t)
	ctx := context.Background()

	if _, err := us.UpdateUser(ctx, &model.Event{
		UserID:     "",
		Type:       model.EventPageView,
		PageURL:    "https://example.com/",
		DeviceType: model.DeviceMobile,
		OS:         "Android",
		Browser:    "Chrome",
		Country:    "US",
	}); err != nil {
		t.Fatalf("UpdateUser (anonymous): %v", err)
	}

	ud, err := us.GetUser(ctx, "anonymous-user")
	if err != nil {
		t.Fatalf("GetUser (anonymous): %v", err)
	}
	if ud.OS != "Android" {
		t.Fatalf("expected dimensions under anonymous-user, got OS=%q", ud.OS)
	}
}

// TestUpdateUser_DoesNotMutateStoredPointer verifies the write path clones the
// stored dimension rather than mutating it in place, which would be a data
// race for concurrent readers.
func TestUpdateUser_DoesNotMutateStoredPointer(t *testing.T) {
	us := newTestUserStore(t)
	ctx := context.Background()

	rich := &model.Event{
		UserID:     "u2",
		Type:       model.EventPageView,
		PageURL:    "https://example.com/",
		DeviceType: model.DeviceMobile,
		OS:         "Android",
	}
	if _, err := us.UpdateUser(ctx, rich); err != nil {
		t.Fatalf("UpdateUser: %v", err)
	}

	// Grab the stored pointer (as a concurrent reader would).
	stored, _ := us.index.Get("u2")
	storedOS := stored.OS

	// A sparse event must not mutate the previously returned pointer.
	if _, err := us.UpdateUser(ctx, &model.Event{
		UserID:  "u2",
		Type:    model.EventClick,
		PageURL: "https://example.com/",
	}); err != nil {
		t.Fatalf("UpdateUser (sparse): %v", err)
	}

	if stored.OS != storedOS {
		t.Errorf("stored dimension pointer was mutated in place: OS=%q, want %q", stored.OS, storedOS)
	}
}
