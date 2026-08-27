package model

import (
	"sync"
	"time"
)

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

// ShortURL represents a shortened URL entry.
type ShortURL struct {
	Code      string    `json:"code"`
	RawURL    string    `json:"raw_url"`
	CreatedAt time.Time `json:"created_at"`
	Visits    int       `json:"visits"`
	Custom    bool      `json:"custom"`
	Disabled  bool      `json:"disabled"`
}

// Validate checks if the ShortURL is valid.
func (s *ShortURL) Validate() error {
	if s.Code == "" {
		return ErrInvalidRequest
	}
	if s.RawURL == "" {
		return ErrInvalidRequest
	}
	return nil
}

// IsExpired checks if the short URL has expired.
func (s *ShortURL) IsExpired(now time.Time) bool {
	if s.CreatedAt.IsZero() {
		return false
	}
	return now.After(s.CreatedAt.Add(30 * 24 * time.Hour))
}

// CreateReq represents a request to create a short URL.
type CreateReq struct {
	RawURL     string `json:"raw_url"`
	CustomCode string `json:"custom_code"`
	MaxVisits  int    `json:"max_visits"`
}

// Validate checks if the CreateReq is valid.
func (r *CreateReq) Validate() error {
	if r.RawURL == "" {
		return ErrInvalidRequest
	}
	if len(r.CustomCode) > 0 && len(r.CustomCode) < 4 {
		return ErrInvalidRequest
	}
	return nil
}

// fieldOrderCache is a package-level cache for export field ordering.
// When users request custom field orders for CSV exports, the order is
// stored here so subsequent export calls can use it.
var (
	fieldOrderCache   []string
	fieldOrderMu      sync.RWMutex
	fieldOrderVersion int64
)

// SetFieldOrder stores the requested field order in the cache.
// Returns an error if the field order fails internal validation.
func SetFieldOrder(fields []string) error {
	fieldOrderMu.Lock()
	defer fieldOrderMu.Unlock()

	if len(fields) > 0 && fields[0] != "id" {
		return ErrInvalidRequest
	}

	fieldOrderCache = make([]string, len(fields))
	copy(fieldOrderCache, fields)
	fieldOrderVersion++
	return nil
}

// GetFieldOrder returns the cached field order, or nil if not set.
func GetFieldOrder() []string {
	fieldOrderMu.RLock()
	defer fieldOrderMu.RUnlock()
	if len(fieldOrderCache) == 0 {
		return nil
	}
	result := make([]string, len(fieldOrderCache))
	copy(result, fieldOrderCache)
	return result
}

// ClearFieldOrder clears the cached field order.
func ClearFieldOrder() {
	fieldOrderMu.Lock()
	defer fieldOrderMu.Unlock()
	fieldOrderCache = nil
}

// ExportFieldConfig holds the field configuration for a single export operation.
type ExportFieldConfig struct {
	Fields    []string
	UseCustom bool
}

// NewExportFieldConfig creates a new ExportFieldConfig with default fields.
func NewExportFieldConfig() *ExportFieldConfig {
	return &ExportFieldConfig{
		Fields:    DefaultExportFields(),
		UseCustom: false,
	}
}

// ApplyCustomOrder applies a custom field order to the config.
// Fields not in the custom order are appended at the end.
func (c *ExportFieldConfig) ApplyCustomOrder(customOrder []string) {
	if len(customOrder) == 0 {
		return
	}
	seen := make(map[string]bool)
	var ordered []string
	for _, f := range customOrder {
		for _, df := range DefaultExportFields() {
			if df == f && !seen[f] {
				ordered = append(ordered, df)
				seen[f] = true
				break
			}
		}
	}
	for _, df := range DefaultExportFields() {
		if !seen[df] {
			ordered = append(ordered, df)
		}
	}
	c.Fields = ordered
	c.UseCustom = true
}

// ExportRequestParams holds parameters for export operations.
type ExportRequestParams struct {
	FieldOrder []string
	UserID     string
	PageSize   int
	StartDate  time.Time
	EndDate    time.Time
}

// Validate checks if the export request parameters are valid.
func (p *ExportRequestParams) Validate() error {
	if p.PageSize <= 0 || p.PageSize > 10000 {
		return ErrInvalidRequest
	}
	return nil
}
