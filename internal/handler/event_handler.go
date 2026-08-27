// Package handler provides HTTP request handlers for the API endpoints.
// Each handler is responsible for parsing request parameters, calling
// the appropriate service, and formatting the response.
package handler

import (
	"encoding/json"
	"fmt"
	"net/http"
	"strconv"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/response"
)

// EventHandler handles HTTP requests for event operations.
type EventHandler struct {
	service *service.EventService
	logger  *logger.Logger
}

// NewEventHandler creates a new EventHandler.
func NewEventHandler(svc *service.EventService, log *logger.Logger) *EventHandler {
	return &EventHandler{
		service: svc,
		logger:  log,
	}
}

// CreateEvent handles POST /api/events - creates a new tracking event.
func (h *EventHandler) CreateEvent(w http.ResponseWriter, r *http.Request) {
	var req model.EventCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		h.logger.Warnf("Invalid event request: %v", err)
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	// Validate required fields
	if err := validateEventRequest(&req); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	event, err := h.service.CreateEvent(r.Context(), &req)
	if err != nil {
		h.logger.Errorf("Failed to create event: %v", err)
		response.WriteError(w, err)
		return
	}

	response.Created(w, event)
}

// CreateEvents handles POST /api/events/batch - creates multiple events.
func (h *EventHandler) CreateEvents(w http.ResponseWriter, r *http.Request) {
	var reqs []*model.EventCreateRequest
	if err := json.NewDecoder(r.Body).Decode(&reqs); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	if len(reqs) == 0 {
		response.BadRequest(w, "at least one event is required")
		return
	}

	if len(reqs) > 1000 {
		response.BadRequest(w, "maximum 1000 events per batch")
		return
	}

	events, err := h.service.CreateEvents(r.Context(), reqs)
	if err != nil {
		h.logger.Errorf("Failed to create events: %v", err)
		response.WriteError(w, err)
		return
	}

	response.Created(w, map[string]interface{}{
		"count":  len(events),
		"events": events,
	})
}

// GetEvent handles GET /api/events/{id} - retrieves a single event.
func (h *EventHandler) GetEvent(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "event ID is required")
		return
	}

	event, err := h.service.GetEvent(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, event)
}

// ListEvents handles GET /api/events - lists events with pagination and filtering.
func (h *EventHandler) ListEvents(w http.ResponseWriter, r *http.Request) {
	query := buildEventQuery(r)
	if err := query.Validate(); err != nil {
		response.BadRequest(w, err.Error())
		return
	}

	events, total, err := h.service.ListEvents(r.Context(), query)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Paginated(w, events, total, query.Page, query.PageSize)
}

// DeleteEvent handles DELETE /api/events/{id} - deletes an event.
func (h *EventHandler) DeleteEvent(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "event ID is required")
		return
	}

	if err := h.service.DeleteEvent(r.Context(), id); err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]string{"message": "event deleted"})
}

// GetEventStats handles GET /api/events/stats - returns aggregated event statistics.
func (h *EventHandler) GetEventStats(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	stats, err := h.service.GetEventStats(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, stats)
}

// validateEventRequest validates event request parameters.
func validateEventRequest(req *model.EventCreateRequest) error {
	if req.UserID == "" {
		return fmt.Errorf("user_id is required")
	}
	if req.Type == "" || !req.Type.Valid() {
		return fmt.Errorf("valid event type is required: page_view, click, duration, conversion, custom")
	}
	if req.PageURL == "" {
		return fmt.Errorf("page_url is required")
	}
	return nil
}

// buildEventQuery builds an EventQuery from HTTP request parameters.
func buildEventQuery(r *http.Request) model.EventQuery {
	q := r.URL.Query()
	query := model.EventQuery{
		UserID:     q.Get("user_id"),
		SessionID:  q.Get("session_id"),
		Page:       parseIntParam(q, "page", 1),
		PageSize:   parseIntParam(q, "page_size", 50),
		SortBy:     q.Get("sort_by"),
		SortOrder:  q.Get("sort_order"),
	}

	if t := q.Get("type"); t != "" {
		query.Type = model.EventType(t)
	}
	if d := q.Get("device_type"); d != "" {
		query.DeviceType = model.DeviceType(d)
	}
	if s := q.Get("start_date"); s != "" {
		query.StartDate, _ = time.Parse(time.RFC3339, s)
	}
	if e := q.Get("end_date"); e != "" {
		query.EndDate, _ = time.Parse(time.RFC3339, e)
	}

	return query
}

// parseIntParam parses an integer query parameter with a default value.
func parseIntParam(q map[string][]string, name string, defaultVal int) int {
	vals, ok := q[name]
	if !ok || len(vals) == 0 {
		return defaultVal
	}
	v, err := strconv.Atoi(vals[0])
	if err != nil {
		return defaultVal
	}
	return v
}

// parseTimeRange parses start_date and end_date from query parameters.
func parseTimeRange(r *http.Request) (time.Time, time.Time) {
	q := r.URL.Query()
	var start, end time.Time

	if s := q.Get("start_date"); s != "" {
		start, _ = time.Parse(time.RFC3339, s)
	}
	if e := q.Get("end_date"); e != "" {
		end, _ = time.Parse(time.RFC3339, e)
	}

	return start, end
}

// extractPathParam extracts a path parameter from the request URL.
func extractPathParam(r *http.Request, _ string) string {
	// Parse the path to extract the ID after /api/events/
	path := r.URL.Path
	for i := len(path) - 1; i >= 0; i-- {
		if path[i] == '/' {
			return path[i+1:]
		}
	}
	return ""
}
