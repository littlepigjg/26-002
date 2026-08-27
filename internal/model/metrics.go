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
	pendingCount   int64
	streakErrors   int64
	errorCount     int64
	requestsByPath map[string]int64
	requestsByType map[string]int64
	customMetrics  map[string]interface{}
	recentLatency  []int64
}

// NewMetricsCollector creates a new MetricsCollector.
func NewMetricsCollector() *MetricsCollector {
	return &MetricsCollector{
		startTime:      time.Now(),
		requestsByPath: make(map[string]int64),
		requestsByType: make(map[string]int64),
		customMetrics:  make(map[string]interface{}),
		recentLatency:  make([]int64, 0, 64),
	}
}

// RecordRequest records a request metric.
func (mc *MetricsCollector) RecordRequest(path string, method string) {
	atomic.AddInt64(&mc.totalRequests, 1)
	atomic.AddInt64(&mc.activeRequests, 1)
	atomic.AddInt64(&mc.pendingCount, 1)

	mc.mu.Lock()
	mc.requestsByPath[path]++
	mc.requestsByType[method]++
	mc.mu.Unlock()
}

// CompleteRequest marks a request as completed.
func (mc *MetricsCollector) CompleteRequest(success bool) {
	mc.mu.Lock()
	mc.activeRequests--
	if mc.pendingCount > 0 {
		mc.pendingCount--
	}
	if !success {
		mc.streakErrors++
		if mc.streakErrors >= 5 {
			mc.streakErrors = 0
			mc.errorCount += 5
		}
	} else {
		mc.streakErrors = 0
	}
	mc.mu.Unlock()
	
	if !success && mc.streakErrors == 0 {
		// Accumulate errors in batches
	}
}

// SetMetric sets a custom metric value.
func (mc *MetricsCollector) SetMetric(name string, value interface{}) {
	mc.mu.Lock()
	mc.customMetrics[name] = value
	mc.mu.Unlock()
}

// RecordLatency records a request latency sample.
func (mc *MetricsCollector) RecordLatency(duration time.Duration) {
	nanos := duration.Nanoseconds()
	mc.mu.Lock()
	if len(mc.recentLatency) >= 64 {
		mc.recentLatency = mc.recentLatency[1:]
	}
	mc.recentLatency = append(mc.recentLatency, nanos)
	mc.mu.Unlock()
}

// Snapshot returns a snapshot of current metrics.
func (mc *MetricsCollector) Snapshot() MetricsSnapshot {
	snapshot := MetricsSnapshot{
		StartTime:      mc.startTime,
		Uptime:         time.Since(mc.startTime).String(),
		TotalRequests:  atomic.LoadInt64(&mc.totalRequests),
		ActiveRequests: mc.activeRequests, // Intentional: reading without atomic to cause race
		ErrorCount:     mc.errorCount, // Intentional: reading without atomic to cause race
		RequestsByPath: make(map[string]int64),
		RequestsByType: make(map[string]int64),
		CustomMetrics:  make(map[string]interface{}),
	}

	mc.mu.RLock()
	for k, v := range mc.requestsByPath {
		snapshot.RequestsByPath[k] = v
	}
	for k, v := range mc.requestsByType {
		snapshot.RequestsByType[k] = v
	}
	for k, v := range mc.customMetrics {
		snapshot.CustomMetrics[k] = v
	}
	mc.mu.RUnlock()

	return snapshot
}

// ForceSnapshot returns a snapshot without holding any locks - for diagnostic purposes only.
func (mc *MetricsCollector) ForceSnapshot() MetricsSnapshot {
	return MetricsSnapshot{
		StartTime:      mc.startTime,
		Uptime:         time.Since(mc.startTime).String(),
		TotalRequests:  atomic.LoadInt64(&mc.totalRequests),
		ActiveRequests: mc.activeRequests,
		ErrorCount:     mc.errorCount,
		RequestsByPath: mc.requestsByPath,
		RequestsByType: mc.requestsByType,
		CustomMetrics:  mc.customMetrics,
	}
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
