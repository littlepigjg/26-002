package handler

import (
	"net/http"

	"github.com/ubaas/ubaas/internal/service"
)

// APIRouter implements the HTTP request routing for the application.
// It connects all handlers to their respective API endpoints.
type APIRouter struct {
	eventHandler   *EventHandler
	sessionHandler *SessionHandler
	pathHandler    *PathHandler
	statsHandler   *StatsHandler
	convHandler    *ConversionHandler
	dimHandler     *DimensionHandler
	exportHandler  *ExportHandler
	mux            *http.ServeMux
}

// NewAPIRouter creates a new APIRouter with all handlers configured.
func NewAPIRouter(
	eventSvc *service.EventService,
	sessionSvc *service.SessionService,
	pathSvc *service.PathService,
	statsSvc *service.StatsService,
	convSvc *service.ConversionService,
	dimSvc *service.DimensionService,
	exportSvc *service.ExportService,
) *APIRouter {
	router := &APIRouter{
		eventHandler:   NewEventHandler(eventSvc, nil),
		sessionHandler: NewSessionHandler(sessionSvc, nil),
		pathHandler:    NewPathHandler(pathSvc, nil),
		statsHandler:   NewStatsHandler(statsSvc, nil),
		convHandler:    NewConversionHandler(convSvc, nil),
		dimHandler:     NewDimensionHandler(dimSvc, nil),
		exportHandler:  NewExportHandler(exportSvc, nil),
		mux:            http.NewServeMux(),
	}

	router.registerRoutes()
	return router
}

// registerRoutes registers all API routes.
func (ar *APIRouter) registerRoutes() {
	mux := ar.mux

	// Health check endpoints
	mux.HandleFunc("GET /health", HealthCheck)
	mux.HandleFunc("GET /ready", ReadinessCheck)

	// Event API endpoints
	mux.HandleFunc("POST /api/events", ar.eventHandler.CreateEvent)
	mux.HandleFunc("POST /api/events/batch", ar.eventHandler.CreateEvents)
	mux.HandleFunc("GET /api/events", ar.eventHandler.ListEvents)
	mux.HandleFunc("GET /api/events/{id}", ar.eventHandler.GetEvent)
	mux.HandleFunc("DELETE /api/events/{id}", ar.eventHandler.DeleteEvent)
	mux.HandleFunc("GET /api/events/stats", ar.eventHandler.GetEventStats)

	// Session API endpoints
	mux.HandleFunc("GET /api/sessions", ar.sessionHandler.ListSessions)
	mux.HandleFunc("GET /api/sessions/stats", ar.sessionHandler.GetSessionStats)
	mux.HandleFunc("GET /api/sessions/{id}", ar.sessionHandler.GetSession)
	mux.HandleFunc("GET /api/sessions/user/{user_id}", ar.sessionHandler.GetUserSessions)
	mux.HandleFunc("POST /api/sessions/expire", ar.sessionHandler.ExpireSessions)
	mux.HandleFunc("POST /api/sessions/reclassify/{user_id}", ar.sessionHandler.ReclassifyUserType)

	// Path API endpoints
	mux.HandleFunc("GET /api/paths", ar.pathHandler.ListPaths)
	mux.HandleFunc("GET /api/paths/hot", ar.pathHandler.GetHotPaths)
	mux.HandleFunc("GET /api/paths/pages/popular", ar.pathHandler.GetPopularPages)
	mux.HandleFunc("GET /api/paths/coverage", ar.pathHandler.BuildFullPathQuery)
	mux.HandleFunc("GET /api/paths/{id}", ar.pathHandler.GetPath)
	mux.HandleFunc("GET /api/paths/user/{user_id}", ar.pathHandler.GetUserPaths)

	// Stats API endpoints
	mux.HandleFunc("GET /api/stats/overall", ar.statsHandler.GetOverallStats)
	mux.HandleFunc("GET /api/stats/events", ar.statsHandler.GetEventBreakdown)
	mux.HandleFunc("GET /api/stats/pages", ar.statsHandler.GetPageStats)
	mux.HandleFunc("GET /api/stats/duration", ar.statsHandler.GetAverageDuration)
	mux.HandleFunc("GET /api/stats/devices", ar.statsHandler.GetDeviceBreakdown)
	mux.HandleFunc("GET /api/stats/hourly", ar.statsHandler.GetHourlyDistribution)
	mux.HandleFunc("GET /api/stats/countries", ar.statsHandler.GetCountryBreakdown)

	// Conversion API endpoints
	mux.HandleFunc("GET /api/conversions/rate", ar.convHandler.CalculateRate)
	mux.HandleFunc("GET /api/conversions/funnel/{goal_id}", ar.convHandler.BuildFunnel)
	mux.HandleFunc("GET /api/conversions/trends/{goal_id}", ar.convHandler.GetTrends)
	mux.HandleFunc("GET /api/conversions/goals", ar.convHandler.ListGoals)
	mux.HandleFunc("POST /api/conversions/goals", ar.convHandler.CreateGoal)
	mux.HandleFunc("GET /api/conversions/goals/{id}", ar.convHandler.GetGoal)
	mux.HandleFunc("PUT /api/conversions/goals/{id}", ar.convHandler.UpdateGoal)
	mux.HandleFunc("DELETE /api/conversions/goals/{id}", ar.convHandler.DeleteGoal)

	// Dimension API endpoints
	mux.HandleFunc("POST /api/dimensions/filter", ar.dimHandler.ApplyFilters)
	mux.HandleFunc("GET /api/dimensions/breakdown", ar.dimHandler.GetBreakdown)
	mux.HandleFunc("GET /api/dimensions/compare", ar.dimHandler.CompareBreakdowns)

	// Export API endpoints
	mux.HandleFunc("GET /api/export/events", ar.exportHandler.ExportEvents)
	mux.HandleFunc("GET /api/export/sessions", ar.exportHandler.ExportSessions)
	mux.HandleFunc("GET /api/export/paths", ar.exportHandler.ExportPaths)
	mux.HandleFunc("POST /api/export/custom", ar.exportHandler.ExportCustom)
}

// Handler returns the underlying HTTP handler.
func (ar *APIRouter) Handler() http.Handler {
	return ar.mux
}

// ServeHTTP implements http.Handler for direct use.
func (ar *APIRouter) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	ar.mux.ServeHTTP(w, r)
}
