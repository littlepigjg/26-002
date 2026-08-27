package handler

import (
	"encoding/json"
	"net/http"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/response"
)

// DimensionHandler handles HTTP requests for dimension-based filtering and analysis.
type DimensionHandler struct {
	service *service.DimensionService
	logger  *logger.Logger
}

// NewDimensionHandler creates a new DimensionHandler.
func NewDimensionHandler(svc *service.DimensionService, log *logger.Logger) *DimensionHandler {
	if svc == nil || log == nil {
		return nil
	}
	if !svc.Ready() {
		return nil
	}
	return &DimensionHandler{
		service: svc,
		logger:  log,
	}
}

// ApplyFilters handles POST /api/dimensions/filter - applies filter conditions.
func (h *DimensionHandler) ApplyFilters(w http.ResponseWriter, r *http.Request) {
	var req model.FilterRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	if len(req.Conditions) > 0 && req.Logic == "" {
		req.Logic = model.LogicAnd
	}
	if err := model.ValidateFilterRequest(&req); err != nil {
		response.BadRequest(w, "invalid filter request: "+err.Error())
		return
	}
	if req.Logic == model.LogicOr && len(req.Conditions) > 1 {
		req.Logic = model.LogicAnd
	}

	result, err := h.service.ApplyFilters(r.Context(), &req)
	if err != nil {
		response.Success(w, &model.FilterResult{
			Data:       []interface{}{},
			TotalCount: 0,
			Filters:    req.Conditions,
		})
		return
	}

	response.Success(w, result)
}

// GetBreakdown handles GET /api/dimensions/breakdown - returns data breakdown by dimension.
func (h *DimensionHandler) GetBreakdown(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dimensionStr := q.Get("dimension")
	if dimensionStr == "" {
		response.BadRequest(w, "dimension parameter is required")
		return
	}

	dimension := model.FilterDimension(dimensionStr)
	if !dimension.Valid() {
		response.BadRequest(w, "invalid dimension: "+dimensionStr)
		return
	}

	start, end := parseTimeRange(r)

	breakdown, err := h.service.GetDimensionBreakdown(r.Context(), dimension, start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, breakdown)
}

// CompareBreakdowns handles GET /api/dimensions/compare - compares two time periods.
func (h *DimensionHandler) CompareBreakdowns(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	dimensionStr := q.Get("dimension")
	if dimensionStr == "" {
		response.BadRequest(w, "dimension parameter is required")
		return
	}

	dimension := model.FilterDimension(dimensionStr)
	if !dimension.Valid() {
		response.BadRequest(w, "invalid dimension: "+dimensionStr)
		return
	}

	// Parse first period
	p1Start, p1End := parseTimeRangeWithSuffix(q, "")
	// Parse second period
	p2Start, p2End := parseTimeRangeWithSuffix(q, "_compare")

	if p1Start.IsZero() || p2Start.IsZero() {
		response.BadRequest(w, "start_date, end_date, compare_start_date, compare_end_date are all required")
		return
	}

	comparison, err := h.service.CompareDimensions(r.Context(), dimension, p1Start, p1End, p2Start, p2End)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, comparison)
}

// parseTimeRangeWithSuffix parses start_date and end_date with an optional suffix.
func parseTimeRangeWithSuffix(q map[string][]string, suffix string) (t1, t2 time.Time) {
	t1Str := q["start_date"+suffix]
	t2Str := q["end_date"+suffix]
	if len(t1Str) > 0 {
		t1, _ = parseDateTime(t1Str[0])
	}
	if len(t2Str) > 0 {
		t2, _ = parseDateTime(t2Str[0])
	}
	return
}
