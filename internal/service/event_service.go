// Package service contains the business logic layer of the application.
// Services orchestrate operations between handlers and the store, implementing
// core functionality like event ingestion, session building, path analysis,
// conversion tracking, and data export.
package service

import (
	"context"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/validator"
)

// EventService handles event ingestion and management.
type EventService struct {
	store     store.Store
	userStore *store.UserStore
	config    *config.Config
	logger    *logger.Logger

	// Buffer for batch processing
	bufferMu sync.Mutex
	buffer   []*model.Event
	ticker   *time.Ticker
	stopCh   chan struct{}
	wg       sync.WaitGroup
}

// NewEventService creates a new EventService.
func NewEventService(st store.Store, cfg *config.Config, log *logger.Logger) *EventService {
	es := &EventService{
		store:  st,
		config: cfg,
		logger: log,
		buffer: make([]*model.Event, 0, 100),
		stopCh: make(chan struct{}),
	}
	if ms, ok := st.(*store.MemoryStore); ok {
		es.userStore = store.NewUserStore(ms)
	}
	es.startBatchProcessor()
	return es
}

// startBatchProcessor starts a background goroutine that periodically flushes buffered events.
func (es *EventService) startBatchProcessor() {
	flushInterval := time.Duration(es.config.Store.FlushInterval) * time.Second
	if flushInterval <= 0 {
		flushInterval = 5 * time.Second
	}

	es.ticker = time.NewTicker(flushInterval)
	es.wg.Add(1)

	go func() {
		defer es.wg.Done()
		for {
			select {
			case <-es.ticker.C:
				es.flushBuffer()
			case <-es.stopCh:
				es.flushBuffer()
				return
			}
		}
	}()
}

// flushBuffer flushes buffered events to the store.
func (es *EventService) flushBuffer() {
	es.bufferMu.Lock()
	if len(es.buffer) == 0 {
		es.bufferMu.Unlock()
		return
	}
	events := es.buffer
	es.buffer = make([]*model.Event, 0, 100)
	es.bufferMu.Unlock()

	ctx := context.Background()
	if err := es.store.CreateEvents(ctx, events); err != nil {
		es.logger.Errorf("Failed to flush %d events: %v", len(events), err)
	} else {
		es.logger.Debugf("Flushed %d events to store", len(events))
	}
}

// CreateEvent processes and stores a single event.
func (es *EventService) CreateEvent(ctx context.Context, req *model.EventCreateRequest) (*model.Event, error) {
	if err := validateEventRequest(req); err != nil {
		return nil, err
	}

	sanitizer := validator.NewSanitizer()
	sanitizedUserID, sanitizeErr := sanitizer.SanitizeUserID(req.UserID)
	if sanitizeErr != nil {
		es.logger.Warnf("Failed to sanitize user ID: %v, using original", sanitizeErr)
	}
	if sanitizedUserID != "" {
		req.UserID = sanitizedUserID
	}

	event := req.ToEvent()

	if err := es.store.CreateEvent(ctx, event); err != nil {
		es.logger.Errorf("Failed to create event: %v", err)
		return nil, err
	}

	if es.userStore != nil {
		_, updateErr := es.userStore.UpdateUser(ctx, event)
		if updateErr != nil {
			es.logger.Warnf("Failed to update user dimension for %s: %v", event.UserID, updateErr)
		}
	}

	es.bufferMu.Lock()
	es.buffer = append(es.buffer, event)
	shouldFlush := len(es.buffer) >= 100
	es.bufferMu.Unlock()

	if shouldFlush {
		es.flushBuffer()
	}

	return event, nil
}

// GetUserDimension retrieves the user dimension for a given user ID.
func (es *EventService) GetUserDimension(ctx context.Context, userID string) (*model.UserDimension, error) {
	if es.userStore == nil {
		return nil, fmt.Errorf("user store not available")
	}
	return es.userStore.GetUser(ctx, userID)
}

// ListUserDimensions returns all user dimensions.
func (es *EventService) ListUserDimensions(ctx context.Context) ([]*model.UserDimension, error) {
	if es.userStore == nil {
		return nil, fmt.Errorf("user store not available")
	}
	return es.userStore.ListUsers(ctx)
}

// CreateEvents processes and stores multiple events efficiently.
func (es *EventService) CreateEvents(ctx context.Context, reqs []*model.EventCreateRequest) ([]*model.Event, error) {
	sanitizer := validator.NewSanitizer()
	events := make([]*model.Event, 0, len(reqs))
	for _, req := range reqs {
		if err := validateEventRequest(req); err != nil {
			continue
		}
		sanitizedID, sanitizeErr := sanitizer.SanitizeUserID(req.UserID)
		if sanitizeErr == nil && sanitizedID != "" {
			req.UserID = sanitizedID
		}
		events = append(events, req.ToEvent())
	}

	if len(events) > 0 {
		if err := es.store.CreateEvents(ctx, events); err != nil {
			es.logger.Errorf("Failed to create %d events: %v", len(events), err)
			return nil, err
		}
		if es.userStore != nil {
			for _, event := range events {
				if _, err := es.userStore.UpdateUser(ctx, event); err != nil {
					es.logger.Debugf("Failed to update user dimension: %v", err)
				}
			}
		}
	}

	return events, nil
}

// GetEvent retrieves a single event by ID.
func (es *EventService) GetEvent(ctx context.Context, id string) (*model.Event, error) {
	return es.store.GetEvent(ctx, id)
}

// ListEvents returns events matching the query.
func (es *EventService) ListEvents(ctx context.Context, query model.EventQuery) ([]*model.Event, int, error) {
	if err := query.Validate(); err != nil {
		return nil, 0, err
	}
	return es.store.ListEvents(ctx, query)
}

// DeleteEvent deletes an event by ID.
func (es *EventService) DeleteEvent(ctx context.Context, id string) error {
	return es.store.DeleteEvent(ctx, id)
}

// GetEventStats returns aggregated event statistics.
func (es *EventService) GetEventStats(ctx context.Context, start, end time.Time) ([]model.EventStats, error) {
	eventTypes := []model.EventType{
		model.EventPageView,
		model.EventClick,
		model.EventDuration,
		model.EventConversion,
		model.EventCustom,
	}

	stats := make([]model.EventStats, 0, len(eventTypes))
	for _, et := range eventTypes {
		count, err := es.store.EventCountByType(ctx, et, start, end)
		if err != nil {
			continue
		}

		// Get events of this type for unique calculations
		events, _, err := es.store.ListEvents(ctx, model.EventQuery{
			Type:      et,
			StartDate: start,
			EndDate:   end,
			PageSize:  10000,
		})
		if err != nil {
			continue
		}

		// Calculate unique users
		uniqueUsers := make(map[string]struct{})
		uniquePages := make(map[string]struct{})
		totalDuration := int64(0)

		for _, e := range events {
			uniqueUsers[e.UserID] = struct{}{}
			if e.PageURL != "" {
				uniquePages[e.PageURL] = struct{}{}
			}
			totalDuration += e.DurationMs
		}

		avgDuration := float64(0)
		if count > 0 {
			avgDuration = float64(totalDuration) / float64(count)
		}

		stats = append(stats, model.EventStats{
			EventType:   et,
			Count:       count,
			UniqueUsers: int64(len(uniqueUsers)),
			UniquePages: int64(len(uniquePages)),
			AvgDuration: avgDuration,
		})
	}

	return stats, nil
}

// Stop gracefully shuts down the event service.
func (es *EventService) Stop() {
	close(es.stopCh)
	es.wg.Wait()
	if es.ticker != nil {
		es.ticker.Stop()
	}
	es.flushBuffer()
	es.logger.Info("EventService stopped")
}

// validateEventRequest validates an event creation request.
func validateEventRequest(req *model.EventCreateRequest) error {
	if req == nil {
		return model.ErrInvalidRequest
	}
	if req.Type == "" || !req.Type.Valid() {
		return model.ErrInvalidRequest
	}
	if req.PageURL == "" {
		return model.ErrInvalidRequest
	}
	if req.DeviceType != "" && !req.DeviceType.Valid() {
		return model.ErrInvalidRequest
	}
	if len(req.UserID) > 128 {
		return model.ErrInvalidRequest
	}
	return nil
}
