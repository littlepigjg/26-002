package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// ExportService handles data export in various formats.
type ExportService struct {
	store  store.Store
	config *config.Config
	logger *logger.Logger
}

// NewExportService creates a new ExportService.
func NewExportService(st store.Store, cfg *config.Config, log *logger.Logger) *ExportService {
	return &ExportService{
		store:  st,
		config: cfg,
		logger: log,
	}
}

// ExportEvents exports events matching the given query in the specified format.
func (es *ExportService) ExportEvents(ctx context.Context, query model.EventQuery, format model.ExportFormat) ([]byte, string, error) {
	if !format.Valid() {
		return nil, "", model.ErrInvalidRequest
	}

	// Limit the number of records
	maxRecords := es.config.Export.MaxRecords
	if query.PageSize <= 0 || query.PageSize > maxRecords {
		query.PageSize = maxRecords
	}

	events, total, err := es.store.ListEvents(ctx, query)
	if err != nil {
		return nil, "", err
	}

	if total > maxRecords {
		return nil, "", fmt.Errorf("%w: %d records exceed limit of %d", model.ErrTooManyRecords, total, maxRecords)
	}

	var data []byte
	var contentType string

	switch format {
	case model.ExportJSON:
		data, err = es.exportJSON(events)
		contentType = "application/json"
	case model.ExportCSV:
		data, err = es.exportCSV(events)
		contentType = "text/csv"
	case model.ExportNDJSON:
		data, err = es.exportNDJSON(events)
		contentType = "application/x-ndjson"
	default:
		return nil, "", model.ErrInvalidRequest
	}

	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", model.ErrExportFailed, err)
	}

	return data, contentType, nil
}

// exportJSON exports events as a JSON array.
func (es *ExportService) exportJSON(events []*model.Event) ([]byte, error) {
	export := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"total":       len(events),
		"events":      events,
	}
	return json.MarshalIndent(export, "", "  ")
}

// exportCSV exports events as CSV.
func (es *ExportService) exportCSV(events []*model.Event) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	// Write header
	header := []string{
		"id", "user_id", "session_id", "type", "page_url", "page_title",
		"duration_ms", "referrer", "device_type", "os", "browser",
		"country", "timestamp",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	// Write rows
	for _, e := range events {
		row := []string{
			e.ID,
			e.UserID,
			e.SessionID,
			string(e.Type),
			e.PageURL,
			e.PageTitle,
			fmt.Sprintf("%d", e.DurationMs),
			e.Referrer,
			string(e.DeviceType),
			e.OS,
			e.Browser,
			e.Country,
			e.Timestamp.Format(time.RFC3339),
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	if err := writer.Error(); err != nil {
		return nil, err
	}

	return buf.Bytes(), nil
}

// exportNDJSON exports events as newline-delimited JSON.
func (es *ExportService) exportNDJSON(events []*model.Event) ([]byte, error) {
	var buf bytes.Buffer
	encoder := json.NewEncoder(&buf)

	for _, e := range events {
		if err := encoder.Encode(e); err != nil {
			return nil, err
		}
	}

	return buf.Bytes(), nil
}

// ExportSessions exports sessions in the specified format.
func (es *ExportService) ExportSessions(ctx context.Context, query model.SessionQuery, format model.ExportFormat) ([]byte, string, error) {
	if !format.Valid() {
		return nil, "", model.ErrInvalidRequest
	}

	maxRecords := es.config.Export.MaxRecords
	if query.PageSize <= 0 || query.PageSize > maxRecords {
		query.PageSize = maxRecords
	}

	sessions, total, err := es.store.ListSessions(ctx, query)
	if err != nil {
		return nil, "", err
	}

	if total > maxRecords {
		return nil, "", fmt.Errorf("%w: %d records exceed limit of %d", model.ErrTooManyRecords, total, maxRecords)
	}

	var data []byte
	var contentType string

	switch format {
	case model.ExportJSON:
		data, err = es.exportSessionsJSON(sessions)
		contentType = "application/json"
	case model.ExportCSV:
		data, err = es.exportSessionsCSV(sessions)
		contentType = "text/csv"
	default:
		return nil, "", model.ErrInvalidRequest
	}

	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", model.ErrExportFailed, err)
	}

	return data, contentType, nil
}

// exportSessionsJSON exports sessions as JSON.
func (es *ExportService) exportSessionsJSON(sessions []*model.Session) ([]byte, error) {
	export := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"total":       len(sessions),
		"sessions":    sessions,
	}
	return json.MarshalIndent(export, "", "  ")
}

// exportSessionsCSV exports sessions as CSV.
func (es *ExportService) exportSessionsCSV(sessions []*model.Session) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{
		"id", "user_id", "user_type", "device_type", "state",
		"start_time", "end_time", "last_event_time", "event_count",
		"total_duration_ms", "referrer", "country",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, s := range sessions {
		row := []string{
			s.ID,
			s.UserID,
			string(s.UserType),
			string(s.DeviceType),
			string(s.State),
			s.StartTime.Format(time.RFC3339),
			s.EndTime.Format(time.RFC3339),
			s.LastEventTime.Format(time.RFC3339),
			fmt.Sprintf("%d", s.EventCount),
			fmt.Sprintf("%d", s.TotalDuration),
			s.Referrer,
			s.Country,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), nil
}

// ExportPaths exports path sequences in the specified format.
func (es *ExportService) ExportPaths(ctx context.Context, query model.PathQuery, format model.ExportFormat) ([]byte, string, error) {
	if !format.Valid() {
		return nil, "", model.ErrInvalidRequest
	}

	paths, err := es.store.ListPaths(ctx, query)
	if err != nil {
		return nil, "", err
	}

	var data []byte
	var contentType string

	switch format {
	case model.ExportJSON:
		data, err = es.exportPathsJSON(paths)
		contentType = "application/json"
	case model.ExportCSV:
		data, err = es.exportPathsCSV(paths)
		contentType = "text/csv"
	default:
		return nil, "", model.ErrInvalidRequest
	}

	if err != nil {
		return nil, "", fmt.Errorf("%w: %v", model.ErrExportFailed, err)
	}

	return data, contentType, nil
}

// exportPathsJSON exports paths as JSON.
func (es *ExportService) exportPathsJSON(paths []*model.PathSequence) ([]byte, error) {
	export := map[string]interface{}{
		"exported_at": time.Now().Format(time.RFC3339),
		"total":       len(paths),
		"paths":       paths,
	}
	return json.MarshalIndent(export, "", "  ")
}

// exportPathsCSV exports paths as CSV.
func (es *ExportService) exportPathsCSV(paths []*model.PathSequence) ([]byte, error) {
	var buf bytes.Buffer
	writer := csv.NewWriter(&buf)

	header := []string{
		"id", "user_id", "session_id", "length", "start_time", "end_time", "path",
	}
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, p := range paths {
		pathStr := ""
		for i, node := range p.Nodes {
			if i > 0 {
				pathStr += " → "
			}
			pathStr += node.PageURL
		}

		row := []string{
			p.ID,
			p.UserID,
			p.SessionID,
			fmt.Sprintf("%d", p.Length),
			p.StartTime.Format(time.RFC3339),
			p.EndTime.Format(time.RFC3339),
			pathStr,
		}
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), nil
}
