package handler

import (
	"net/http"

	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
	"github.com/ubaas/ubaas/pkg/response"
)

// StatsHandler handles HTTP requests for statistics operations.
type StatsHandler struct {
	service *service.StatsService
	logger  *logger.Logger
}

// NewStatsHandler creates a new StatsHandler.
func NewStatsHandler(svc *service.StatsService, log *logger.Logger) *StatsHandler {
	return &StatsHandler{
		service: svc,
		logger:  log,
	}
}

// GetOverallStats handles GET /api/stats/overall - returns overall statistics.
func (h *StatsHandler) GetOverallStats(w http.ResponseWriter, r *http.Request) {
	stats, err := h.service.GetOverallStats(r.Context())
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, stats)
}

// GetEventBreakdown handles GET /api/stats/events - returns event breakdown by type.
func (h *StatsHandler) GetEventBreakdown(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	breakdown, err := h.service.GetEventBreakdown(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, breakdown)
}

// GetPageStats handles GET /api/stats/pages - returns page statistics.
func (h *StatsHandler) GetPageStats(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)
	limit := parseIntParam(r.URL.Query(), "limit", 20)

	pages, err := h.service.GetPageStats(r.Context(), start, end, limit)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, pages)
}

// GetAverageDuration handles GET /api/stats/duration - returns average duration by page.
func (h *StatsHandler) GetAverageDuration(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	durations, err := h.service.GetAverageDuration(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, durations)
}

// GetDeviceBreakdown handles GET /api/stats/devices - returns device breakdown.
func (h *StatsHandler) GetDeviceBreakdown(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	breakdown, err := h.service.GetDeviceBreakdown(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, breakdown)
}

// GetHourlyDistribution handles GET /api/stats/hourly - returns hourly distribution.
func (h *StatsHandler) GetHourlyDistribution(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	hourly, err := h.service.GetHourlyDistribution(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, map[string]interface{}{
		"hourly_distribution": hourly,
		"total":               sumHourly(hourly),
	})
}

// GetCountryBreakdown handles GET /api/stats/countries - returns country breakdown.
func (h *StatsHandler) GetCountryBreakdown(w http.ResponseWriter, r *http.Request) {
	start, end := parseTimeRange(r)

	countries, err := h.service.GetCountryBreakdown(r.Context(), start, end)
	if err != nil {
		response.WriteError(w, err)
		return
	}

	response.Success(w, countries)
}

// sumHourly calculates the total count across all hours.
func sumHourly(hourly map[int]int64) int64 {
	var total int64
	for _, count := range hourly {
		total += count
	}
	return total
}
