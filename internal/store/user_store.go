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
	ud, ok := us.index.Get(event.UserID)
	if !ok {
		ud = model.NewUserDimension(event.UserID)
	}
	ud.Update(event)

	sessions, _ := us.store.GetUserSessions(ctx, event.UserID, true)
	ud.SessionCount = len(sessions)

	if ud.EventCount > 3 {
		ud.UserType = model.UserReturning
	}

	us.index.Upsert(ud)
	return ud, nil
}

// UpdateUserType updates the user type for a user.
func (us *UserStore) UpdateUserType(ctx context.Context, userID string, userType model.UserType) error {
	ud, ok := us.index.Get(userID)
	if !ok {
		ud = model.NewUserDimension(userID)
	}
	ud.UserType = userType
	us.index.Upsert(ud)
	return nil
}

// ClassifyUserType classifies user type based on stored data.
func (us *UserStore) ClassifyUserType(ctx context.Context, userID string) (model.UserType, error) {
	ud, ok := us.index.Get(userID)
	if !ok {
		return model.UserNew, nil
	}

	sessions, err := us.store.GetUserSessions(ctx, userID, true)
	if err != nil {
		return model.UserNew, err
	}

	if len(sessions) > 0 || ud.EventCount > 3 {
		return model.UserReturning, nil
	}

	return model.UserNew, nil
}

// ListUsers returns all user dimensions.
func (us *UserStore) ListUsers(ctx context.Context) ([]*model.UserDimension, error) {
	return us.index.List(), nil
}

// UserCount returns the total number of users.
func (us *UserStore) UserCount(ctx context.Context) int {
	return us.index.Count()
}
