package handler

import (
	"net/http"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/response"
)

// PathHandler handles HTTP requests for path analysis operations.
type PathHandler struct {
	service *service.PathService
	logger  *logger.Logger
}

// NewPathHandler creates a new PathHandler.
func NewPathHandler(svc *service.PathService, log *logger.Logger) *PathHandler {
	return &PathHandler{
		service: svc,
		logger:  log,
	}
}

// GetPath handles GET /api/paths/{id} - retrieves a path sequence by ID.
func (h *PathHandler) GetPath(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "path ID is required")
		return
	}

	path, err := h.service.GetPathSequence(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, path)
}

// GetUserPaths handles GET /api/paths/user/{user_id} - lists paths for a user.
func (h *PathHandler) GetUserPaths(w http.ResponseWriter, r *http.Request) {
	userID := extractPathParam(r, "user_id")
	if userID == "" {
		response.BadRequest(w, "user ID is required")
		return
	}

	paths, err := h.service.GetUserPaths(r.Context(), userID)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, paths)
}

// ListPaths handles GET /api/paths - lists paths with filtering.
func (h *PathHandler) ListPaths(w http.ResponseWriter, r *http.Request) {
	query := buildPathQuery(r)
	paths, err := h.service.ListPaths(r.Context(), query)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"total": len(paths),
		"paths": paths,
	})
}

// GetHotPaths handles GET /api/paths/hot - returns the most frequent path patterns.
func (h *PathHandler) GetHotPaths(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)
	limit := parseIntParam(r.URL.Query(), "limit", 20)

	hotPaths, err := h.service.GetHotPaths(r.Context(), start, end, limit)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"hot_paths": hotPaths,
		"count":     len(hotPaths),
	})
}

// GetPopularPages handles GET /api/paths/pages/popular - returns popular pages.
func (h *PathHandler) GetPopularPages(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)
	limit := parseIntParam(r.URL.Query(), "limit", 20)

	pages, err := h.service.GetPopularPages(r.Context(), start, end, limit)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"popular_pages": pages,
		"count":         len(pages),
	})
}

// buildPathQuery builds a PathQuery from HTTP request parameters.
func buildPathQuery(r *http.Request) model.PathQuery {
	q := r.URL.Query()
	query := model.PathQuery{
		Limit: parseIntParam(q, "limit", 20),
	}

	if s := q.Get("start_date"); s != "" {
		query.StartDate, _ = parseDateTime(s)
	}
	if e := q.Get("end_date"); e != "" {
		query.EndDate, _ = parseDateTime(e)
	}
	if ml := q.Get("min_length"); ml != "" {
		query.MinLength = parseIntParam(q, "min_length", 0)
	}
	if ml := q.Get("max_length"); ml != "" {
		query.MaxLength = parseIntParam(q, "max_length", 0)
	}
	if sp := q.Get("start_page"); sp != "" {
		query.StartPage = sp
	}
	if ep := q.Get("end_page"); ep != "" {
		query.EndPage = ep
	}
	if dt := q.Get("device_type"); dt != "" {
		query.DeviceType = model.DeviceType(dt)
	}

	return query
}

// BuildFullPathQuery handles GET /api/paths/coverage - calculates path coverage.
func (h *PathHandler) BuildFullPathQuery(w http.ResponseWriter, r *http.Request) {
	pathStr := r.URL.Query().Get("path")
	if pathStr == "" {
		response.BadRequest(w, "path parameter is required")
		return
	}

	start, end := parseTimeRange(r)
	if start.IsZero() {
		start = time.Now().Add(-24 * time.Hour)
	}
	if end.IsZero() {
		end = time.Now()
	}

	steps, err := h.service.ComputePathCoverage(r.Context(), pathStr, start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"path":  pathStr,
		"steps": steps,
		"period": map[string]string{
			"start": start.Format("2006-01-02T15:04:05Z07:00"),
			"end":   end.Format("2006-01-02T15:04:05Z07:00"),
		},
	})
}
