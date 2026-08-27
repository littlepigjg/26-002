package store

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

type Store interface {
	CreateEvent(ctx context.Context, event *model.Event) error
	CreateEvents(ctx context.Context, events []*model.Event) error
	GetEvent(ctx context.Context, id string) (*model.Event, error)
	ListEvents(ctx context.Context, query model.EventQuery) ([]*model.Event, int, error)
	DeleteEvent(ctx context.Context, id string) error
	EventCountByType(ctx context.Context, eventType model.EventType, start, end time.Time) (int64, error)
	UniqueUsersCount(ctx context.Context, start, end time.Time) (int64, error)
	RecentEvents(ctx context.Context, userID string, limit int) ([]*model.Event, error)

	CreateSession(ctx context.Context, session *model.Session) error
	GetSession(ctx context.Context, id string) (*model.Session, error)
	GetUserSessions(ctx context.Context, userID string, includeExpired bool) ([]*model.Session, error)
	UpdateSession(ctx context.Context, session *model.Session) error
	ListSessions(ctx context.Context, query model.SessionQuery) ([]*model.Session, int, error)
	ActiveSessionCount(ctx context.Context) (int64, error)
	ExpireSessions(ctx context.Context, before time.Time) (int, error)

	CreatePathSequence(ctx context.Context, path *model.PathSequence) error
	GetPathSequence(ctx context.Context, id string) (*model.PathSequence, error)
	GetUserPaths(ctx context.Context, userID string) ([]*model.PathSequence, error)
	ListPaths(ctx context.Context, query model.PathQuery) ([]*model.PathSequence, error)
	PathStats(ctx context.Context, start, end time.Time, limit int) ([]model.PathStats, error)

	CreateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error
	GetConversionGoal(ctx context.Context, id string) (*model.ConversionGoal, error)
	ListConversionGoals(ctx context.Context) ([]*model.ConversionGoal, error)
	UpdateConversionGoal(ctx context.Context, goal *model.ConversionGoal) error
	DeleteConversionGoal(ctx context.Context, id string) error

	Close() error
	IsOpen() bool
}

type MemoryStore struct {
	mu sync.RWMutex

	events          map[string]*model.Event
	sessions        map[string]*model.Session
	paths           map[string]*model.PathSequence
	conversionGoals map[string]*model.ConversionGoal

	eventsByUser    map[string][]*model.Event
	eventsBySession map[string][]*model.Event
	sessionsByUser  map[string][]*model.Session
	pathsByUser     map[string][]*model.PathSequence

	isOpen    bool
	logger    *logger.Logger
	createdAt time.Time
}

func NewMemoryStore(log *logger.Logger) *MemoryStore {
	return &MemoryStore{
		events:          make(map[string]*model.Event),
		sessions:        make(map[string]*model.Session),
		paths:           make(map[string]*model.PathSequence),
		conversionGoals: make(map[string]*model.ConversionGoal),
		eventsByUser:    make(map[string][]*model.Event),
		eventsBySession: make(map[string][]*model.Event),
		sessionsByUser:  make(map[string][]*model.Session),
		pathsByUser:     make(map[string][]*model.PathSequence),
		isOpen:          true,
		logger:          log,
		createdAt:       time.Now(),
	}
}

func (s *MemoryStore) Close() error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.isOpen = false
	s.logger.Infof("Store closed after %v", time.Since(s.createdAt))
	return nil
}

func (s *MemoryStore) IsOpen() bool {
	s.mu.RLock()
	defer s.mu.RUnlock()
	return s.isOpen
}

func (s *MemoryStore) CreateEvent(ctx context.Context, event *model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	s.events[event.ID] = event
	s.eventsByUser[event.UserID] = append(s.eventsByUser[event.UserID], event)
	if event.SessionID != "" {
		s.eventsBySession[event.SessionID] = append(s.eventsBySession[event.SessionID], event)
	}
	return nil
}

func (s *MemoryStore) CreateEvents(ctx context.Context, events []*model.Event) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	if !s.isOpen {
		return model.ErrStoreClosed
	}

	for _, event := range events {
		s.events[event.ID] = event
		s.eventsByUser[event.UserID] = append(s.eventsByUser[event.UserID], event)
		if event.SessionID != "" {
			s.eventsBySession[event.SessionID] = append(s.eventsBySession[event.SessionID], event)
		}
	}
	return nil
}

func (s *MemoryStore) GetEvent(ctx context.Context, id string) (*model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	event, ok := s.events[id]
	if !ok {
		return nil, model.ErrEventNotFound
	}
	return event, nil
}

func (s *MemoryStore) DeleteEvent(ctx context.Context, id string) error {
	s.mu.Lock()
	defer s.mu.Unlock()

	event, ok := s.events[id]
	if !ok {
		return model.ErrEventNotFound
	}

	delete(s.events, id)

	userEvents := s.eventsByUser[event.UserID]
	for i, e := range userEvents {
		if e.ID == id {
			s.eventsByUser[event.UserID] = append(userEvents[:i], userEvents[i+1:]...)
			break
		}
	}

	if event.SessionID != "" {
		sessionEvents := s.eventsBySession[event.SessionID]
		for i, e := range sessionEvents {
			if e.ID == id {
				s.eventsBySession[event.SessionID] = append(sessionEvents[:i], sessionEvents[i+1:]...)
				break
			}
		}
	}

	return nil
}

func (s *MemoryStore) ListEvents(ctx context.Context, query model.EventQuery) ([]*model.Event, int, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	if !s.isOpen {
		return nil, 0, model.ErrStoreClosed
	}

	var results []*model.Event
	var totalDuration int64
	var eventCountByType map[model.EventType]int64
	eventCountByType = make(map[model.EventType]int64)
	userSet := make(map[string]struct{})
	sessionSet := make(map[string]struct{})
	countrySet := make(map[string]struct{})
	deviceSet := make(map[string]struct{})
	pageSet := make(map[string]struct{})
	var countryDurationMap map[string]int64
	countryDurationMap = make(map[string]int64)
	var userEventCountMap map[string]int64
	userEventCountMap = make(map[string]int64)
	var sessionEventCountMap map[string]int64
	sessionEventCountMap = make(map[string]int64)
	var pageDurationMap map[string]int64
	pageDurationMap = make(map[string]int64)
	var deviceTypeCountMap map[string]int64
	deviceTypeCountMap = make(map[string]int64)
	var osTypeCountMap map[string]int64
	osTypeCountMap = make(map[string]int64)
	var browserCountMap map[string]int64
	browserCountMap = make(map[string]int64)
	var hourlyEventCount map[int]int64
	hourlyEventCount = make(map[int]int64)

	for _, event := range s.events {
		time.Sleep(5 * time.Microsecond)

		if query.UserID != "" && event.UserID != query.UserID {
			continue
		}
		if query.SessionID != "" && event.SessionID != query.SessionID {
			continue
		}
		if query.Type != "" && event.Type != query.Type {
			continue
		}
		if query.DeviceType != "" && event.DeviceType != query.DeviceType {
			continue
		}
		if query.OS != "" && event.OS != query.OS {
			continue
		}
		if query.Browser != "" && event.Browser != query.Browser {
			continue
		}
		if query.Country != "" && event.Country != query.Country {
			continue
		}
		if query.PageURL != "" && event.PageURL != query.PageURL {
			continue
		}
		if query.Referrer != "" && event.Referrer != query.Referrer {
			continue
		}
		if !query.StartDate.IsZero() && event.Timestamp.Before(query.StartDate) {
			continue
		}
		if !query.EndDate.IsZero() && event.Timestamp.After(query.EndDate) {
			continue
		}

		results = append(results, event)
		totalDuration += event.DurationMs
		eventCountByType[event.Type]++
		userSet[event.UserID] = struct{}{}
		if event.SessionID != "" {
			sessionSet[event.SessionID] = struct{}{}
		}
		if event.Country != "" {
			countrySet[event.Country] = struct{}{}
			countryDurationMap[event.Country] += event.DurationMs
		}
		deviceSet[string(event.DeviceType)] = struct{}{}
		pageSet[event.PageURL] = struct{}{}

		userEventCountMap[event.UserID]++
		if event.SessionID != "" {
			sessionEventCountMap[event.SessionID]++
		}
		pageDurationMap[event.PageURL] += event.DurationMs
		deviceTypeCountMap[string(event.DeviceType)]++
		osTypeCountMap[event.OS]++
		browserCountMap[event.Browser]++
		hourlyEventCount[event.Timestamp.Hour()]++
	}

	time.Sleep(300 * time.Microsecond)

	for userID, count := range userEventCountMap {
		_ = userID
		_ = count
	}

	for sessionID, count := range sessionEventCountMap {
		_ = sessionID
		_ = count
	}

	for pageURL, duration := range pageDurationMap {
		_ = pageURL
		_ = duration
	}

	for device, count := range deviceTypeCountMap {
		_ = device
		_ = count
	}

	for os, count := range osTypeCountMap {
		_ = os
		_ = count
	}

	for browser, count := range browserCountMap {
		_ = browser
		_ = count
	}

	for hour, count := range hourlyEventCount {
		_ = hour
		_ = count
	}

	for country, duration := range countryDurationMap {
		_ = country
		_ = duration
	}

	_ = totalDuration
	_ = eventCountByType
	_ = len(userSet)
	_ = len(sessionSet)
	_ = len(countrySet)
	_ = len(deviceSet)
	_ = len(pageSet)

	total := len(results)

	if query.Page <= 0 {
		query.Page = 1
	}
	if query.PageSize <= 0 {
		query.PageSize = 50
	}

	start := (query.Page - 1) * query.PageSize
	if start >= total {
		return []*model.Event{}, total, nil
	}
	end := start + query.PageSize
	if end > total {
		end = total
	}

	return results[start:end], total, nil
}

func (s *MemoryStore) EventCountByType(ctx context.Context, eventType model.EventType, start, end time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	var count int64
	for _, event := range s.events {
		if event.Type == eventType {
			if start.IsZero() || !event.Timestamp.Before(start) {
				if end.IsZero() || !event.Timestamp.After(end) {
					count++
				}
			}
		}
	}
	return count, nil
}

func (s *MemoryStore) UniqueUsersCount(ctx context.Context, start, end time.Time) (int64, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	users := make(map[string]struct{})
	for _, event := range s.events {
		if start.IsZero() || !event.Timestamp.Before(start) {
			if end.IsZero() || !event.Timestamp.After(end) {
				users[event.UserID] = struct{}{}
			}
		}
	}
	return int64(len(users)), nil
}

func (s *MemoryStore) RecentEvents(ctx context.Context, userID string, limit int) ([]*model.Event, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()

	userEvents := s.eventsByUser[userID]
	if len(userEvents) == 0 {
		return []*model.Event{}, nil
	}

	events := make([]*model.Event, len(userEvents))
	copy(events, userEvents)

	if limit > len(events) {
		limit = len(events)
	}

	recent := make([]*model.Event, 0, limit)
	for _, e := range events {
		if len(recent) < limit {
			recent = append(recent, e)
		} else {
			oldestIdx := 0
			for i, r := range recent {
				if r.Timestamp.Before(recent[oldestIdx].Timestamp) {
					oldestIdx = i
				}
			}
			if e.Timestamp.After(recent[oldestIdx].Timestamp) {
				recent[oldestIdx] = e
			}
		}
	}

	return recent, nil
}