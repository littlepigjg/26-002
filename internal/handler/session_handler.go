package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/response"
)

// SessionHandler handles HTTP requests for session operations.
type SessionHandler struct {
	service *service.SessionService
	logger  *logger.Logger
}

// NewSessionHandler creates a new SessionHandler.
func NewSessionHandler(svc *service.SessionService, log *logger.Logger) *SessionHandler {
	return &SessionHandler{
		service: svc,
		logger:  log,
	}
}

// GetSession handles GET /api/sessions/{id} - retrieves a session by ID.
func (h *SessionHandler) GetSession(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "session ID is required")
		return
	}

	session, err := h.service.GetSession(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, session)
}

// GetUserSessions handles GET /api/sessions/user/{user_id} - lists sessions for a user.
func (h *SessionHandler) GetUserSessions(w http.ResponseWriter, r *http.Request) {
	userID := extractPathParam(r, "user_id")
	if userID == "" {
		response.BadRequest(w, "user ID is required")
		return
	}

	// Check if we should include expired sessions
	includeExpired := r.URL.Query().Get("include_expired") == "true"

	sessions, err := h.service.GetUserSessions(r.Context(), userID, includeExpired)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, sessions)
}

// ListSessions handles GET /api/sessions - lists sessions with filtering.
func (h *SessionHandler) ListSessions(w http.ResponseWriter, r *http.Request) {
	query := buildSessionQuery(r)
	sessions, total, err := h.service.ListSessions(r.Context(), query)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Paginated(w, sessions, total, query.Page, query.PageSize)
}

// GetSessionStats handles GET /api/sessions/stats - returns session statistics.
func (h *SessionHandler) GetSessionStats(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	stats, err := h.service.GetSessionStats(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, stats)
}

// ExpireSessions handles POST /api/sessions/expire - manually expires sessions.
func (h *SessionHandler) ExpireSessions(w http.ResponseWriter, r *http.Request) {
	var req struct {
		Before string `json:"before"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	before, err := parseDateTime(req.Before)
	if err != nil {
		response.BadRequest(w, "invalid date format: "+err.Error())
		return
	}

	count, err := h.service.ExpireSessions(r.Context(), before)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"expired_count": count,
		"before":        before.Format("2006-01-02T15:04:05Z07:00"),
	})
}

// ReclassifyUserType handles POST /api/sessions/reclassify/{user_id}.
func (h *SessionHandler) ReclassifyUserType(w http.ResponseWriter, r *http.Request) {
	userID := extractPathParam(r, "user_id")
	if userID == "" {
		response.BadRequest(w, "user ID is required")
		return
	}

	if err := h.service.ReclassifyUserType(r.Context(), userID); err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]string{"message": "user type reclassified"})
}

// buildSessionQuery builds a SessionQuery from HTTP request parameters.
func buildSessionQuery(r *http.Request) model.SessionQuery {
	q := r.URL.Query()
	query := model.SessionQuery{
		UserID:   q.Get("user_id"),
		Page:     parseIntParam(q, "page", 1),
		PageSize: parseIntParam(q, "page_size", 50),
	}

	if s := q.Get("state"); s != "" {
		query.State = model.SessionState(s)
	}
	if ut := q.Get("user_type"); ut != "" {
		query.UserType = model.UserType(ut)
	}
	if dt := q.Get("device_type"); dt != "" {
		query.DeviceType = model.DeviceType(dt)
	}
	if s := q.Get("start_date"); s != "" {
		query.StartDate, _ = parseDateTime(s)
	}
	if e := q.Get("end_date"); e != "" {
		query.EndDate, _ = parseDateTime(e)
	}
	if d := q.Get("min_duration_ms"); d != "" {
		query.MinDurationMs, _ = parseInt64(d)
	}

	return query
}
