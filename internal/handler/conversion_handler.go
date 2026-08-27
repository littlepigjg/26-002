package handler

import (
	"encoding/json"
	"net/http"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/response"
)

// ConversionHandler handles HTTP requests for conversion operations.
type ConversionHandler struct {
	service *service.ConversionService
	logger  *logger.Logger
}

// NewConversionHandler creates a new ConversionHandler.
func NewConversionHandler(svc *service.ConversionService, log *logger.Logger) *ConversionHandler {
	return &ConversionHandler{
		service: svc,
		logger:  log,
	}
}

// CreateGoal handles POST /api/conversions/goals - creates a new conversion goal.
func (h *ConversionHandler) CreateGoal(w http.ResponseWriter, r *http.Request) {
	var goal model.ConversionGoal
	if err := json.NewDecoder(r.Body).Decode(&goal); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	if goal.Name == "" {
		response.BadRequest(w, "name is required")
		return
	}
	if goal.StartPage == "" {
		response.BadRequest(w, "start_page is required")
		return
	}
	if goal.EndPage == "" {
		response.BadRequest(w, "end_page is required")
		return
	}

	if err := h.service.CreateConversionGoal(r.Context(), &goal); err != nil {
		response.WriteError(w, err)
		return
	}

	response.Created(w, goal)
}

// GetGoal handles GET /api/conversions/goals/{id} - retrieves a conversion goal.
func (h *ConversionHandler) GetGoal(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "goal ID is required")
		return
	}

	goal, err := h.service.GetConversionGoal(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, goal)
}

// ListGoals handles GET /api/conversions/goals - lists all conversion goals.
func (h *ConversionHandler) ListGoals(w http.ResponseWriter, r *http.Request) {
	goals, err := h.service.ListConversionGoals(r.Context())
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, goals)
}

// UpdateGoal handles PUT /api/conversions/goals/{id} - updates a conversion goal.
func (h *ConversionHandler) UpdateGoal(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "goal ID is required")
		return
	}

	goal, err := h.service.GetConversionGoal(r.Context(), id)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	var updates map[string]interface{}
	if err := json.NewDecoder(r.Body).Decode(&updates); err != nil {
		response.BadRequest(w, "invalid request body: "+err.Error())
		return
	}

	// Apply updates
	if name, ok := updates["name"].(string); ok {
		goal.Name = name
	}
	if sp, ok := updates["start_page"].(string); ok {
		goal.StartPage = sp
	}
	if ep, ok := updates["end_page"].(string); ok {
		goal.EndPage = ep
	}
	if desc, ok := updates["description"].(string); ok {
		goal.Description = desc
	}

	if err := h.service.UpdateConversionGoal(r.Context(), goal); err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, goal)
}

// DeleteGoal handles DELETE /api/conversions/goals/{id} - deletes a conversion goal.
func (h *ConversionHandler) DeleteGoal(w http.ResponseWriter, r *http.Request) {
	id := extractPathParam(r, "id")
	if id == "" {
		response.BadRequest(w, "goal ID is required")
		return
	}

	if err := h.service.DeleteConversionGoal(r.Context(), id); err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]string{"message": "goal deleted"})
}

// CalculateRate handles GET /api/conversions/rate - calculates conversion rate.
func (h *ConversionHandler) CalculateRate(w http.ResponseWriter, r *http.Request) {
	query := buildConversionQuery(r)

	result, err := h.service.CalculateConversionRate(r.Context(), query)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, result)
}

// BuildFunnel handles GET /api/conversions/funnel/{goal_id} - builds funnel analysis.
func (h *ConversionHandler) BuildFunnel(w http.ResponseWriter, r *http.Request) {
	goalID := extractPathParam(r, "goal_id")
	if goalID == "" {
		response.BadRequest(w, "goal ID is required")
		return
	}

	start, end := parseTimeRange(r)
	if start.IsZero() || end.IsZero() {
		response.BadRequest(w, "start_date and end_date are required")
		return
	}

	analysis, err := h.service.BuildFunnelAnalysis(r.Context(), goalID, start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, analysis)
}

// GetTrends handles GET /api/conversions/trends/{goal_id} - returns conversion trends.
func (h *ConversionHandler) GetTrends(w http.ResponseWriter, r *http.Request) {
	goalID := extractPathParam(r, "goal_id")
	if goalID == "" {
		response.BadRequest(w, "goal ID is required")
		return
	}

	days := parseIntParam(r.URL.Query(), "days", 7)
	if days <= 0 {
		days = 7
	}

	trends, err := h.service.GetConversionTrends(r.Context(), goalID, days)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, trends)
}

// buildConversionQuery builds a ConversionQuery from HTTP request parameters.
func buildConversionQuery(r *http.Request) model.ConversionQuery {
	q := r.URL.Query()
	query := model.ConversionQuery{
		GoalID:    q.Get("goal_id"),
		StartPage: q.Get("start_page"),
		EndPage:   q.Get("end_page"),
	}

	if s := q.Get("start_date"); s != "" {
		query.StartDate, _ = parseDateTime(s)
	}
	if e := q.Get("end_date"); e != "" {
		query.EndDate, _ = parseDateTime(e)
	}
	if d := q.Get("device_type"); d != "" {
		query.DeviceType = model.DeviceType(d)
	}
	if ut := q.Get("user_type"); ut != "" {
		query.UserType = model.UserType(ut)
	}

	return query
}
