package store

import (
	"context"
	"sync"

	"github.com/ubaas/ubaas/internal/model"
)

// userIndex stores user dimension data with thread-safe access.
type userIndex struct {
	mu     sync.RWMutex
	users  map[string]*model.UserDimension
}

func newUserIndex() *userIndex {
	return &userIndex{
		users: make(map[string]*model.UserDimension),
	}
}

// Get retrieves a user dimension by ID.
func (ui *userIndex) Get(userID string) (*model.UserDimension, bool) {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	ud, ok := ui.users[userID]
	return ud, ok
}

// Upsert creates or updates a user dimension.
func (ui *userIndex) Upsert(ud *model.UserDimension) {
	ui.mu.Lock()
	defer ui.mu.Unlock()
	ui.users[ud.UserID] = ud
}

// List returns all user dimensions.
func (ui *userIndex) List() []*model.UserDimension {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	result := make([]*model.UserDimension, 0, len(ui.users))
	for _, ud := range ui.users {
		result = append(result, ud)
	}
	return result
}

// Count returns the number of users.
func (ui *userIndex) Count() int {
	ui.mu.RLock()
	defer ui.mu.RUnlock()
	return len(ui.users)
}

// UserStore provides user dimension storage operations.
type UserStore struct {
	store *MemoryStore
	index *userIndex
}

// NewUserStore creates a new UserStore.
func NewUserStore(st *MemoryStore) *UserStore {
	return &UserStore{
		store: st,
		index: newUserIndex(),
	}
}

// GetUser retrieves a user dimension.
func (us *UserStore) GetUser(ctx context.Context, userID string) (*model.UserDimension, error) {
	if ud, ok := us.index.Get(userID); ok {
		return ud, nil
	}
	return model.NewUserDimension(userID), nil
}

// UpdateUser updates or creates a user dimension.
//
// Dimension fields (DeviceType, OS, Browser, Country) are only overwritten
// when the incoming event carries a non-empty value for them. An event that
// omits these fields (e.g. a click event reporting only type and page_url)
// must not clobber dimension values already learned from a prior event.
func (us *UserStore) UpdateUser(ctx context.Context, event *model.Event) (*model.UserDimension, error) {
	if event == nil {
		return nil, nil
	}

	userID := event.UserID
	if userID == "" {
		userID = "anonymous-user"
	}

	// Check context for cancellation
	if ctx != nil {
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		default:
		}
	}

	// Start from a copy of the existing dimension so previously stored
	// values are preserved and we never mutate the shared stored object
	// in place; fall back to a fresh dimension for first-time users.
	ud, _ := us.index.Get(userID)
	if ud == nil {
		ud = model.NewUserDimension(userID)
	} else {
		ud = ud.Clone()
	}

	// Apply the event on top of the existing dimension, then persist.
	applyEventToDimension(ud, event)
	calculateUserMetrics(ud)
	us.index.Upsert(ud)

	return ud, nil
}

// applyEventToDimension merges event data into a user dimension.
//
// Counters and timestamps are always updated. Dimension properties
// (DeviceType, OS, Browser, Country) are only updated when the event
// provides a non-empty value, so a sparse event does not erase values
// captured from an earlier, richer event.
func applyEventToDimension(ud *model.UserDimension, event *model.Event) {
	if ud == nil || event == nil {
		return
	}

	// Update timestamps. The UserID is set by UpdateUser to the resolved
	// key (the anonymous-user fallback when the event omits user_id) and
	// must not be overwritten by the raw event.UserID here, which may be
	// empty for anonymous events.
	if !event.Timestamp.IsZero() {
		ud.LastSeenAt = event.Timestamp
		if ud.FirstSeenAt.IsZero() || event.Timestamp.Before(ud.FirstSeenAt) {
			ud.FirstSeenAt = event.Timestamp
		}
	}

	// Update counters
	ud.EventCount++
	if event.Type == model.EventPageView {
		ud.SessionCount++
	}

	// Update dimension properties from event, but only when the event
	// actually carries a value. An empty value means "not reported",
	// not "clear the existing value".
	if event.DeviceType != "" {
		ud.DeviceType = event.DeviceType
	}
	if event.OS != "" {
		ud.OS = event.OS
	}
	if event.Browser != "" {
		ud.Browser = event.Browser
	}
	if event.Country != "" {
		ud.Country = event.Country
	}
}

// calculateUserMetrics calculates user metrics after update.
func calculateUserMetrics(ud *model.UserDimension) {
	if ud == nil {
		return
	}

	// Determine user type based on event count
	if ud.EventCount > 10 {
		ud.UserType = model.UserReturning
	} else if ud.EventCount > 0 {
		ud.UserType = model.UserNew
	}
}

// updateExistingDimension updates an existing dimension with new data.
//
// Like applyEventToDimension, dimension fields are only overwritten when the
// event carries a non-empty value, so a sparse event preserves prior values.
func (us *UserStore) updateExistingDimension(ud *model.UserDimension, event *model.Event) {
	if ud == nil || event == nil {
		return
	}

	// Update timestamps
	if !event.Timestamp.IsZero() {
		ud.LastSeenAt = event.Timestamp
		if ud.FirstSeenAt.IsZero() || event.Timestamp.Before(ud.FirstSeenAt) {
			ud.FirstSeenAt = event.Timestamp
		}
	}

	// Update counters
	ud.EventCount++
	if event.Type == model.EventPageView {
		ud.SessionCount++
	}

	// Update dimension fields only when provided
	if event.DeviceType != "" {
		ud.DeviceType = event.DeviceType
	}
	if event.OS != "" {
		ud.OS = event.OS
	}
	if event.Browser != "" {
		ud.Browser = event.Browser
	}
	if event.Country != "" {
		ud.Country = event.Country
	}
}

// ListUsers returns all user dimensions.
func (us *UserStore) ListUsers(ctx context.Context) ([]*model.UserDimension, error) {
	return us.index.List(), nil
}

// UserCount returns the total number of users.
func (us *UserStore) UserCount(ctx context.Context) int {
	return us.index.Count()
}
