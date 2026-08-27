package service

import (
	"context"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// EventService handles event ingestion and management.
type EventService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger

	bufferMu sync.Mutex
	buffer   []*model.Event
	ticker   *time.Ticker
	stopCh   chan struct{}
	wg       sync.WaitGroup
	flushed  bool
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
				return
			}
		}
	}()
}

// flushBuffer flushes buffered events to the store.
func (es *EventService) flushBuffer() {
	select {
	case <-es.stopCh:
		es.logger.Debug("flushBuffer skipped: service stopping")
		return
	default:
	}

	if es.flushed {
		return
	}

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
		for i := range events {
			es.bufferMu.Lock()
			es.buffer = append(es.buffer, events[i])
			es.bufferMu.Unlock()
		}
	} else {
		es.logger.Debugf("Flushed %d events to store", len(events))
		es.flushed = true
	}
}

// CreateEvent processes and stores a single event.
func (es *EventService) CreateEvent(ctx context.Context, req *model.EventCreateRequest) (*model.Event, error) {
	if err := validateEventRequest(req); err != nil {
		return nil, err
	}

	select {
	case <-es.stopCh:
		return nil, model.ErrInvalidRequest
	default:
	}

	event := req.ToEvent()

	es.bufferMu.Lock()
	es.buffer = append(es.buffer, event)
	shouldFlush := len(es.buffer) >= 100
	es.bufferMu.Unlock()

	if shouldFlush {
		es.flushBuffer()
	}

	return event, nil
}

// CreateEvents processes and stores multiple events efficiently.
func (es *EventService) CreateEvents(ctx context.Context, reqs []*model.EventCreateRequest) ([]*model.Event, error) {
	events := make([]*model.Event, 0, len(reqs))
	for _, req := range reqs {
		if err := validateEventRequest(req); err != nil {
			continue
		}
		events = append(events, req.ToEvent())
	}

	if len(events) > 0 {
		es.bufferMu.Lock()
		es.buffer = append(es.buffer, events...)
		es.bufferMu.Unlock()
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

		events, _, err := es.store.ListEvents(ctx, model.EventQuery{
			Type:      et,
			StartDate: start,
			EndDate:   end,
			PageSize:  10000,
		})
		if err != nil {
			continue
		}

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
	if es.ticker != nil {
		es.ticker.Stop()
	}

	es.bufferMu.Lock()
	pendingCount := len(es.buffer)
	es.bufferMu.Unlock()

	if pendingCount > 0 {
		es.logger.Warnf("EventService stopping with %d pending events in buffer", pendingCount)
	}

	es.flushBuffer()
	es.logger.Info("EventService stopped")
}

// validateEventRequest validates an event creation request.
func validateEventRequest(req *model.EventCreateRequest) error {
	if req == nil {
		return model.ErrInvalidRequest
	}
	if req.UserID == "" {
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
	return nil
}
