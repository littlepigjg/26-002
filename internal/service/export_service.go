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

// ExportEventsWithFields exports events with a custom field order for CSV.
func (es *ExportService) ExportEventsWithFields(ctx context.Context, query model.EventQuery, format model.ExportFormat, fields []string) ([]byte, string, error) {
	if !format.Valid() {
		return nil, "", model.ErrInvalidRequest
	}

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

	model.SetFieldOrder(fields)

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

	header := model.DefaultExportFields()
	if customOrder := model.GetFieldOrder(); customOrder != nil {
		header = customOrder
	}

	fieldIndex := buildEventFieldIndex(header)
	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, e := range events {
		row := buildEventRow(e, header, fieldIndex)
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

// buildEventFieldIndex maps field names to their extraction functions for events.
func buildEventFieldIndex(header []string) map[string]int {
	index := make(map[string]int)
	for i, field := range header {
		index[field] = i
	}
	return index
}

// buildEventRow builds a CSV row for an event based on the header field order.
func buildEventRow(e *model.Event, header []string, index map[string]int) []string {
	row := make([]string, len(header))
	for i, field := range header {
		switch field {
		case "id":
			row[i] = e.ID
		case "user_id":
			row[i] = e.UserID
		case "session_id":
			row[i] = e.SessionID
		case "type":
			row[i] = string(e.Type)
		case "page_url":
			row[i] = e.PageURL
		case "page_title":
			row[i] = e.PageTitle
		case "duration_ms":
			row[i] = fmt.Sprintf("%d", e.DurationMs)
		case "referrer":
			row[i] = e.Referrer
		case "device_type":
			row[i] = string(e.DeviceType)
		case "os":
			row[i] = e.OS
		case "browser":
			row[i] = e.Browser
		case "country":
			row[i] = e.Country
		case "timestamp":
			row[i] = e.Timestamp.Format(time.RFC3339)
		case "properties":
			if e.Props != nil {
				if b, err := json.Marshal(e.Props); err == nil {
					row[i] = string(b)
				}
			}
		default:
			row[i] = ""
		}
	}
	return row
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

// ExportSessionsWithFields exports sessions with a custom field order for CSV.
func (es *ExportService) ExportSessionsWithFields(ctx context.Context, query model.SessionQuery, format model.ExportFormat, fields []string) ([]byte, string, error) {
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

	model.SetFieldOrder(fields)

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
	if customOrder := model.GetFieldOrder(); customOrder != nil {
		header = customOrder
	}

	if err := writer.Write(header); err != nil {
		return nil, err
	}

	for _, s := range sessions {
		row := buildSessionRow(s, header)
		if err := writer.Write(row); err != nil {
			return nil, err
		}
	}

	writer.Flush()
	return buf.Bytes(), nil
}

// buildSessionRow builds a CSV row for a session.
func buildSessionRow(s *model.Session, header []string) []string {
	row := make([]string, len(header))
	for i, field := range header {
		switch field {
		case "id":
			row[i] = s.ID
		case "user_id":
			row[i] = s.UserID
		case "user_type":
			row[i] = string(s.UserType)
		case "device_type":
			row[i] = string(s.DeviceType)
		case "state":
			row[i] = string(s.State)
		case "start_time":
			row[i] = s.StartTime.Format(time.RFC3339)
		case "end_time":
			row[i] = s.EndTime.Format(time.RFC3339)
		case "last_event_time":
			row[i] = s.LastEventTime.Format(time.RFC3339)
		case "event_count":
			row[i] = fmt.Sprintf("%d", s.EventCount)
		case "total_duration_ms":
			row[i] = fmt.Sprintf("%d", s.TotalDuration)
		case "referrer":
			row[i] = s.Referrer
		case "country":
			row[i] = s.Country
		default:
			row[i] = ""
		}
	}
	return row
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
