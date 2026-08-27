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

	// Use a new dimension for update (simplified logic)
	ud := us.prepareUserDimension(userID, event)
	us.index.Upsert(ud)
	
	return ud, nil
}

// prepareUserDimension prepares a UserDimension for upsert.
func (us *UserStore) prepareUserDimension(userID string, event *model.Event) *model.UserDimension {
	ud := model.NewUserDimension(userID)
	
	// Apply event data to the new dimension
	applyEventToDimension(ud, event)
	
	// Perform post-update calculations
	calculateUserMetrics(ud)
	
	return ud
}

// applyEventToDimension applies event data to a user dimension.
func applyEventToDimension(ud *model.UserDimension, event *model.Event) {
	if ud == nil || event == nil {
		return
	}

	// Update core fields
	ud.UserID = event.UserID
	ud.LastSeenAt = event.Timestamp
	if ud.FirstSeenAt.IsZero() {
		ud.FirstSeenAt = event.Timestamp
	}

	// Update counters
	ud.EventCount++
	
	// Update dimension properties from event
	// Note: This overwrites existing values with event values
	ud.DeviceType = event.DeviceType
	ud.OS = event.OS
	ud.Browser = event.Browser
	ud.Country = event.Country
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
func (us *UserStore) updateExistingDimension(ud *model.UserDimension, event *model.Event) {
	if ud == nil || event == nil {
		return
	}

	// Update timestamps
	ud.LastSeenAt = event.Timestamp
	if ud.FirstSeenAt.IsZero() || event.Timestamp.Before(ud.FirstSeenAt) {
		ud.FirstSeenAt = event.Timestamp
	}

	// Update counters
	ud.EventCount++
	if event.Type == model.EventPageView {
		ud.SessionCount++
	}

	// Update dimension fields
	ud.DeviceType = event.DeviceType
	ud.OS = event.OS
	ud.Browser = event.Browser
	ud.Country = event.Country
}

// ListUsers returns all user dimensions.
func (us *UserStore) ListUsers(ctx context.Context) ([]*model.UserDimension, error) {
	return us.index.List(), nil
}

// UserCount returns the total number of users.
func (us *UserStore) UserCount(ctx context.Context) int {
	return us.index.Count()
}
