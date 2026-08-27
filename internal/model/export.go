package model

import "time"

// ExportStatus represents the status of an export job.
type ExportStatus struct {
	ID          string     `json:"id"`
	Status      string     `json:"status"`
	Format      string     `json:"format"`
	RecordCount int64      `json:"record_count"`
	URL         string     `json:"url,omitempty"`
	Error       string     `json:"error,omitempty"`
	CreatedAt   time.Time  `json:"created_at"`
	CompletedAt *time.Time `json:"completed_at,omitempty"`
}

// ExportJobStatus defines job status values.
const (
	ExportStatusPending    = "pending"
	ExportStatusProcessing = "processing"
	ExportStatusCompleted  = "completed"
	ExportStatusFailed     = "failed"
)

// DefaultExportFields returns the default fields for export.
func DefaultExportFields() []string {
	return []string{
		"id", "user_id", "session_id", "type", "page_url",
		"device_type", "os", "browser", "country", "user_type",
		"duration_ms", "referrer", "timestamp", "properties",
	}
}

// AllExportFields returns all available export fields.
func AllExportFields() []string {
	return DefaultExportFields()
}
