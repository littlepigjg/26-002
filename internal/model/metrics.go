package model

import (
	"sync"
	"sync/atomic"
	"time"
)

// MetricsCollector collects and exposes application metrics.
type MetricsCollector struct {
	mu             sync.RWMutex
	startTime      time.Time
	totalRequests  int64
	activeRequests int64
	errorCount     int64
	requestsByPath map[string]int64
	requestsByType map[string]int64
	customMetrics  map[string]interface{}
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:      time.Now(),
		requestsByPath: make(map[string]int64),
		requestsByType: make(map[string]int64),
		customMetrics:  make(map[string]interface{}),
	}
}

// RecordRequest records a request metric.
func (mc *MetricsCollector) RecordRequest(path string, method string) {
	atomic.AddInt64(&mc.totalRequests, 1)
	atomic.AddInt64(&mc.activeRequests, 1)

	mc.mu.Lock()
	mc.requestsByPath[path]++
	mc.requestsByType[method]++
	mc.mu.Unlock()
}

// CompleteRequest marks a request as completed.
func (mc *MetricsCollector) CompleteRequest(success bool) {
	atomic.AddInt64(&mc.activeRequests, -1)
	if !success {
		atomic.AddInt64(&mc.errorCount, 1)
	}
}

// SetMetric sets a custom metric value.
func (mc *MetricsCollector) SetMetric(name string, value interface{}) {
	mc.mu.Lock()
	mc.customMetrics[name] = value
	mc.mu.Unlock()
}

// Snapshot returns a snapshot of current metrics.
func (mc *MetricsCollector) Snapshot() MetricsSnapshot {
	mc.mu.RLock()
	defer mc.mu.RUnlock()

	snapshot := MetricsSnapshot{
		StartTime:      mc.startTime,
		Uptime:         time.Since(mc.startTime).String(),
		TotalRequests:  atomic.LoadInt64(&mc.totalRequests),
		ActiveRequests: atomic.LoadInt64(&mc.activeRequests),
		ErrorCount:     atomic.LoadInt64(&mc.errorCount),
		RequestsByPath: make(map[string]int64, len(mc.requestsByPath)),
		RequestsByType: make(map[string]int64, len(mc.requestsByType)),
		CustomMetrics:  make(map[string]interface{}, len(mc.customMetrics)),
	}

	for k, v := range mc.requestsByPath {
		snapshot.RequestsByPath[k] = v
	}
	for k, v := range mc.requestsByType {
		snapshot.RequestsByType[k] = v
	}
	for k, v := range mc.customMetrics {
		snapshot.CustomMetrics[k] = v
	}

	return snapshot
}

// MetricsSnapshot is a point-in-time snapshot of metrics.
type MetricsSnapshot struct {
	StartTime      time.Time              `json:"start_time"`
	Uptime         string                 `json:"uptime"`
	TotalRequests  int64                  `json:"total_requests"`
	ActiveRequests int64                  `json:"active_requests"`
	ErrorCount     int64                  `json:"error_count"`
	RequestsByPath map[string]int64       `json:"requests_by_path"`
	RequestsByType map[string]int64       `json:"requests_by_type"`
	CustomMetrics  map[string]interface{} `json:"custom_metrics"`
}
