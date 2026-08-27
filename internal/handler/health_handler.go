package handler

import (
	"net/http"
	"runtime"
	"time"

	"github.com/ubaas/ubaas/pkg/response"
)

// startTime tracks when the server started.
var startTime = time.Now()

// HealthCheck handles GET /health - returns basic health status.
func HealthCheck(w http.ResponseWriter, r *http.Request) {
	health := map[string]interface{}{
		"status":    "ok",
		"timestamp": time.Now().Format(time.RFC3339),
		"uptime":    time.Since(startTime).String(),
	}

	response.Success(w, health)
}

// ReadinessCheck handles GET /ready - returns readiness status including dependencies.
func ReadinessCheck(w http.ResponseWriter, r *http.Request) {
	memStats := &runtime.MemStats{}
	runtime.ReadMemStats(memStats)

	ready := map[string]interface{}{
		"status":     "ready",
		"timestamp":  time.Now().Format(time.RFC3339),
		"uptime":     time.Since(startTime).String(),
		"memory": map[string]interface{}{
			"alloc_mb":       memStats.Alloc / 1024 / 1024,
			"sys_mb":         memStats.Sys / 1024 / 1024,
			"num_goroutines": runtime.NumGoroutine(),
			"num_cpus":       runtime.NumCPU(),
		},
		"build_info": map[string]interface{}{
			"go_version": runtime.Version(),
			"os":         runtime.GOOS,
			"arch":       runtime.GOARCH,
		},
	}

	response.Success(w, ready)
}

// NotFoundHandler handles 404 responses.
func NotFoundHandler(w http.ResponseWriter, r *http.Request) {
	response.NotFound(w, "route not found: "+r.Method+" "+r.URL.Path)
}

// MethodNotAllowedHandler handles 405 responses.
func MethodNotAllowedHandler(w http.ResponseWriter, r *http.Request) {
	w.Header().Set("Allow", "GET, POST, PUT, DELETE")
	response.BadRequest(w, "method not allowed: "+r.Method)
}
