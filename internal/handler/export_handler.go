package handler

import (
	"encoding/json"
	"fmt"
	"net/http"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/pkg/logger"
)

// ExportHandler handles HTTP requests for data export operations.
type ExportHandler struct {
	service *service.ExportService
	logger  *logger.Logger
}

// NewExportHandler creates a new ExportHandler.
func NewExportHandler(svc *service.ExportService, log *logger.Logger) *ExportHandler {
	return &ExportHandler{
		service: svc,
		logger:  log,
	}
}

// ExportEvents handles GET /api/export/events - exports events.
func (h *ExportHandler) ExportEvents(w http.ResponseWriter, r *http.Request) {
	query := buildEventQuery(r)
	format := model.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = model.ExportJSON
	}

	data, contentType, err := h.service.ExportEvents(r.Context(), query, format)
	if err != nil {
		h.logger.Errorf("Export failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeExportResponse(w, data, contentType, "events", format)
}

// ExportSessions handles GET /api/export/sessions - exports sessions.
func (h *ExportHandler) ExportSessions(w http.ResponseWriter, r *http.Request) {
	query := buildSessionQuery(r)
	format := model.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = model.ExportJSON
	}

	data, contentType, err := h.service.ExportSessions(r.Context(), query, format)
	if err != nil {
		h.logger.Errorf("Export failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeExportResponse(w, data, contentType, "sessions", format)
}

// ExportPaths handles GET /api/export/paths - exports paths.
func (h *ExportHandler) ExportPaths(w http.ResponseWriter, r *http.Request) {
	query := buildPathQuery(r)
	format := model.ExportFormat(r.URL.Query().Get("format"))
	if format == "" {
		format = model.ExportJSON
	}

	data, contentType, err := h.service.ExportPaths(r.Context(), query, format)
	if err != nil {
		h.logger.Errorf("Export failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeExportResponse(w, data, contentType, "paths", format)
}

// ExportCustom handles POST /api/export/custom - handles custom export requests.
func (h *ExportHandler) ExportCustom(w http.ResponseWriter, r *http.Request) {
	var req model.ExportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		http.Error(w, "invalid request body: "+err.Error(), http.StatusBadRequest)
		return
	}

	if !req.Format.Valid() {
		http.Error(w, "invalid export format", http.StatusBadRequest)
		return
	}

	query := model.EventQuery{
		StartDate: req.StartDate,
		EndDate:   req.EndDate,
		PageSize:  req.Options.MaxRecords,
	}

	if query.PageSize <= 0 {
		query.PageSize = 10000
	}

	data, contentType, err := h.service.ExportEvents(r.Context(), query, req.Format)
	if err != nil {
		h.logger.Errorf("Custom export failed: %v", err)
		http.Error(w, err.Error(), http.StatusInternalServerError)
		return
	}

	writeExportResponse(w, data, contentType, "custom", req.Format)
}

// writeExportResponse writes the export response to the HTTP response.
func writeExportResponse(w http.ResponseWriter, data []byte, contentType, resource string, format model.ExportFormat) {
	filename := fmt.Sprintf("%s_export_%s.%s", resource, "timestamp", getFileExtension(format))
	w.Header().Set("Content-Type", contentType)
	w.Header().Set("Content-Disposition", fmt.Sprintf("attachment; filename=%s", filename))
	w.Header().Set("X-Export-Count", fmt.Sprintf("%d", len(data)))
	w.WriteHeader(http.StatusOK)
	w.Write(data)
}

// getFileExtension returns the file extension for an export format.
func getFileExtension(format model.ExportFormat) string {
	switch format {
	case model.ExportJSON:
		return "json"
	case model.ExportCSV:
		return "csv"
	case model.ExportNDJSON:
		return "ndjson"
	default:
		return "txt"
	}
}
