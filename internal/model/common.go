package model

import (
	"crypto/rand"
	"encoding/hex"
	"errors"
	"fmt"
	"sync"
	"time"
)

// Common errors used across the application.
var (
	// ErrEventNotFound is returned when an event is not found.
	ErrEventNotFound = errors.New("event not found")
	// ErrSessionNotFound is returned when a session is not found.
	ErrSessionNotFound = errors.New("session not found")
	// ErrPathNotFound is returned when a path is not found.
	ErrPathNotFound = errors.New("path not found")
	// ErrConversionNotFound is returned when a conversion goal is not found.
	ErrConversionNotFound = errors.New("conversion goal not found")
	// ErrInvalidRequest is returned for invalid request parameters.
	ErrInvalidRequest = errors.New("invalid request")
	// ErrInvalidTimeRange is returned when start > end time.
	ErrInvalidTimeRange = errors.New("start time must be before end time")
	// ErrInvalidMaxLength is returned when max < min length.
	ErrInvalidMaxLength = errors.New("max_length must be >= min_length")
	// ErrMissingGoal is returned when conversion goal is missing.
	ErrMissingGoal = errors.New("conversion goal or start/end page must be specified")
	// ErrInvalidDimension is returned for invalid filter dimensions.
	ErrInvalidDimension = errors.New("invalid filter dimension")
	// ErrInvalidOperator is returned for invalid filter operators.
	ErrInvalidOperator = errors.New("invalid filter operator")
	// ErrStoreClosed is returned when the store is closed.
	ErrStoreClosed = errors.New("store is closed")
	// ErrInvalidState is returned for invalid state transitions.
	ErrInvalidState = errors.New("invalid state transition")
	// ErrExportFailed is returned when export fails.
	ErrExportFailed = errors.New("export failed")
	// ErrTooManyRecords is returned when result set is too large.
	ErrTooManyRecords = errors.New("too many records, please narrow your search")
)

// IDGenerator provides unique ID generation using random hex strings.
type IDGenerator struct {
	mu     sync.Mutex
	prefix string
	counter uint64
}

var defaultIDGenerator = &IDGenerator{
	prefix: "",
}

// NewIDGenerator creates a new IDGenerator with a prefix.
func NewIDGenerator(prefix string) *IDGenerator {
	return &IDGenerator{
		prefix: prefix,
	}
}

// Generate generates a new unique ID.
func (g *IDGenerator) Generate() string {
	g.mu.Lock()
	defer g.mu.Unlock()

	var b [16]byte
	_, _ = rand.Read(b[:])
	hexStr := hex.EncodeToString(b[:])

	if g.prefix != "" {
		return fmt.Sprintf("%s-%s", g.prefix, hexStr)
	}
	return hexStr
}

var (
	eventIDGen   = NewIDGenerator("evt")
	sessionIDGen = NewIDGenerator("ses")
	pathIDGen    = NewIDGenerator("pth")
	goalIDGen    = NewIDGenerator("goal")
	exportIDGen  = NewIDGenerator("exp")
)

func generateEventID() string   { return eventIDGen.Generate() }
func generateSessionID() string { return sessionIDGen.Generate() }
func generatePathID() string    { return pathIDGen.Generate() }
func generateGoalID() string    { return goalIDGen.Generate() }
func generateExportID() string  { return exportIDGen.Generate() }

// RequestContext holds contextual information about a request.
type RequestContext struct {
	RequestID   string    `json:"request_id"`
	UserAgent   string    `json:"user_agent"`
	ClientIP    string    `json:"client_ip"`
	StartedAt   time.Time `json:"started_at"`
	TraceID     string    `json:"trace_id"`
}

// NewRequestContext creates a new RequestContext.
func NewRequestContext(requestID, userAgent, clientIP string) *RequestContext {
	return &RequestContext{
		RequestID: requestID,
		UserAgent: userAgent,
		ClientIP:  clientIP,
		StartedAt: time.Now(),
		TraceID:   generateTraceID(),
	}
}

func generateTraceID() string {
	var b [8]byte
	_, _ = rand.Read(b[:])
	return hex.EncodeToString(b[:])
}

// ExportFormat represents the format for data export.
type ExportFormat string

const (
	// ExportJSON represents JSON export format.
	ExportJSON ExportFormat = "json"
	// ExportCSV represents CSV export format.
	ExportCSV ExportFormat = "csv"
	// ExportNDJSON represents newline-delimited JSON export format.
	ExportNDJSON ExportFormat = "ndjson"
)

// Valid checks if the export format is valid.
func (ef ExportFormat) Valid() bool {
	switch ef {
	case ExportJSON, ExportCSV, ExportNDJSON:
		return true
	default:
		return false
	}
}

// ExportRequest represents a data export request.
type ExportRequest struct {
	Format    ExportFormat     `json:"format"`
	Query     interface{}      `json:"query"`
	StartDate time.Time        `json:"start_date"`
	EndDate   time.Time        `json:"end_date"`
	Options   ExportOptions    `json:"options"`
}

// ExportOptions contains options for data export.
type ExportOptions struct {
	Compress   bool   `json:"compress"`
	MaxRecords int    `json:"max_records"`
	Filename   string `json:"filename"`
}
