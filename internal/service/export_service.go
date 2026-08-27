package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"encoding/json"
	"fmt"
	"sync"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

// ContextSpy is a hook function that intercepts context usage in background tasks.
type ContextSpy func(ctx context.Context)

// JobEntry tracks the state of an asynchronous export job.
type JobEntry struct {
	ID        string
	Status    string
	Progress  int
	Total     int
	Done      chan struct{}
	Cancel    context.CancelFunc
	Canceled  bool
	Error     error
	mu        sync.Mutex
}

// ExportService handles data export in various formats.
type ExportService struct {
	store   store.Store
	config  *config.Config
	logger  *logger.Logger
	spy     ContextSpy
	jobs    map[string]*JobEntry
	jobsMu  sync.Mutex
	wg      sync.WaitGroup
	rootCtx context.Context
}

// NewExportService creates a new ExportService.
func NewExportService(st store.Store, cfg *config.Config, log *logger.Logger) *ExportService {
	return &ExportService{
		store:   st,
		config:  cfg,
		logger:  log,
		jobs:    make(map[string]*JobEntry),
		rootCtx: context.Background(),
	}
}

// SetContextSpy sets a spy function to intercept context usage in background tasks.
func (es *ExportService) SetContextSpy(spy ContextSpy) {
	es.spy = spy
}

// SetRootContext sets the root context for background tasks.
func (es *ExportService) SetRootContext(ctx context.Context) {
	es.rootCtx = ctx
}

// GetJobStatus returns the current status of an export job.
func (es *ExportService) GetJobStatus(jobID string) (*JobEntry, error) {
	es.jobsMu.Lock()
	defer es.jobsMu.Unlock()
	job, ok := es.jobs[jobID]
	if !ok {
		return nil, fmt.Errorf("job not found: %s", jobID)
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	return &JobEntry{
		ID:       job.ID,
		Status:   job.Status,
		Progress: job.Progress,
		Total:    job.Total,
		Canceled: job.Canceled,
		Error:    job.Error,
	}, nil
}

// CancelJob cancels a running export job.
func (es *ExportService) CancelJob(jobID string) error {
	es.jobsMu.Lock()
	job, ok := es.jobs[jobID]
	es.jobsMu.Unlock()
	if !ok {
		return fmt.Errorf("job not found: %s", jobID)
	}
	job.mu.Lock()
	defer job.mu.Unlock()
	if job.Canceled {
		return nil
	}
	job.Canceled = true
	if job.Cancel != nil {
		job.Cancel()
	}
	return nil
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

// ExportAsync starts an asynchronous export job that runs in the background.
// The job processes events matching the query and notifies completion via Done channel.
func (es *ExportService) ExportAsync(ctx context.Context, query model.EventQuery, format model.ExportFormat) (string, error) {
	if !format.Valid() {
		return "", model.ErrInvalidRequest
	}

	jobID := fmt.Sprintf("job-%d", time.Now().UnixNano())
	job := &JobEntry{
		ID:       jobID,
		Status:   model.ExportStatusPending,
		Done:     make(chan struct{}),
		Progress: 0,
		Total:    0,
	}

	es.jobsMu.Lock()
	es.jobs[jobID] = job
	es.jobsMu.Unlock()

	es.wg.Add(1)
	go func() {
		defer es.wg.Done()
		defer close(job.Done)

		job.mu.Lock()
		job.Status = model.ExportStatusProcessing
		job.mu.Unlock()

		// BUG: Use context.Background() instead of deriving from the passed ctx.
		// This means the background task will continue even if the caller's context
		// is canceled (e.g., server shutdown, request timeout).
		// The correct implementation should use: workCtx, cancel := context.WithCancel(ctx)
		workCtx := context.Background()

		if es.spy != nil {
			es.spy(workCtx)
		}

		maxRecords := es.config.Export.MaxRecords
		if query.PageSize <= 0 || query.PageSize > maxRecords {
			query.PageSize = maxRecords
		}

		events, total, err := es.store.ListEvents(workCtx, query)
		if err != nil {
			job.mu.Lock()
			job.Status = model.ExportStatusFailed
			job.Error = err
			job.mu.Unlock()
			return
		}

		job.mu.Lock()
		job.Total = total
		job.mu.Unlock()

		if total > maxRecords {
			job.mu.Lock()
			job.Status = model.ExportStatusFailed
			job.Error = fmt.Errorf("%w: %d records exceed limit of %d", model.ErrTooManyRecords, total, maxRecords)
			job.mu.Unlock()
			return
		}

		// Process events in batches, simulating long-running work
		batchSize := 100
		for i := 0; i < len(events); i += batchSize {
			end := i + batchSize
			if end > len(events) {
				end = len(events)
			}

			batch := events[i:end]

			// BUG: Check context.Background() instead of the caller's context.
			// Even if the caller's ctx is canceled, this won't detect it.
			// The correct implementation should check: select { case <-ctx.Done(): ... }
			select {
			case <-workCtx.Done():
				job.mu.Lock()
				job.Status = model.ExportStatusFailed
				job.Canceled = true
				job.Error = workCtx.Err()
				job.mu.Unlock()
				return
			default:
			}

			// Simulate processing time for realistic export behavior
			time.Sleep(10 * time.Millisecond)

			// Process batch
			for _, e := range batch {
				_ = e.ID
			}

			job.mu.Lock()
			job.Progress = end
			job.mu.Unlock()
		}

		job.mu.Lock()
		job.Status = model.ExportStatusCompleted
		job.mu.Unlock()
	}()

	return jobID, nil
}

// ProcessEventsBatch processes a batch of events and returns the number processed.
// It accepts a context for cancellation support.
func (es *ExportService) ProcessEventsBatch(ctx context.Context, events []*model.Event) (int, error) {
	if len(events) == 0 {
		return 0, nil
	}

	// BUG: The batch processor uses context.Background() for its internal operations
	// instead of the provided context. This means long-running batch operations
	// cannot be canceled via the caller's context.
	// The correct implementation should derive from ctx:
	//   bgCtx, cancel := context.WithCancel(ctx)
	//   defer cancel()
	bgCtx := context.Background()

	if es.spy != nil {
		es.spy(bgCtx)
	}

	processed := 0
	batchSize := 50

	for i := 0; i < len(events); i += batchSize {
		end := i + batchSize
		if end > len(events) {
			end = len(events)
		}

		batch := events[i:end]

		// BUG: Should check ctx.Done() here, not bgCtx.Done()
		select {
		case <-bgCtx.Done():
			return processed, bgCtx.Err()
		default:
		}

		for _, e := range batch {
			if e == nil {
				continue
			}
			// Simulate processing work
			_ = e.Type
			_ = e.PageURL
			processed++
		}
	}

	return processed, nil
}

// Shutdown gracefully stops all background export jobs.
func (es *ExportService) Shutdown(ctx context.Context) error {
	es.logger.Info("Shutting down export service...")

	es.jobsMu.Lock()
	for id, job := range es.jobs {
		job.mu.Lock()
		if job.Status == model.ExportStatusProcessing {
			job.Canceled = true
			job.Cancel()
		}
		job.mu.Unlock()
		delete(es.jobs, id)
	}
	es.jobsMu.Unlock()

	done := make(chan struct{})
	go func() {
		es.wg.Wait()
		close(done)
	}()

	select {
	case <-done:
		es.logger.Info("All export jobs stopped")
		return nil
	case <-ctx.Done():
		es.logger.Warn("Shutdown timeout, forcing stop")
		return ctx.Err()
	}
}
