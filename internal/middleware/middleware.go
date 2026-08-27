// Package middleware provides HTTP middleware for request processing.
// It includes logging, recovery, CORS, and authentication middleware.
package middleware

import (
	"context"
	"net/http"
	"time"

	"github.com/ubaas/ubaas/internal/model"
	"github.com/ubaas/ubaas/pkg/logger"
)

// statusWriter wraps http.ResponseWriter to capture the status code.
type statusWriter struct {
	http.ResponseWriter
	statusCode int
	written    bool
}

// WriteHeader captures the status code.
func (sw *statusWriter) WriteHeader(code int) {
	if !sw.written {
		sw.statusCode = code
		sw.written = true
	}
	sw.ResponseWriter.WriteHeader(code)
}

// LoggingMiddleware logs each HTTP request with method, path, status code, and duration.
func LoggingMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}

			// Add request ID context
			ctx := r.Context()

			next.ServeHTTP(sw, r.WithContext(ctx))

			duration := time.Since(start)
			log.Infof("%s %s %d %v %s",
				r.Method,
				r.URL.Path,
				sw.statusCode,
				duration,
				r.RemoteAddr,
			)
		})
	}
}

// RecoveryMiddleware recovers from panics and returns a 500 error response.
func RecoveryMiddleware(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			defer func() {
				if err := recover(); err != nil {
					log.Errorf("Panic recovered: %v", err)
					http.Error(w, `{"code":500,"message":"internal server error"}`, http.StatusInternalServerError)
				}
			}()
			next.ServeHTTP(w, r)
		})
	}
}

// CORSMiddleware adds CORS headers to allow cross-origin requests.
func CORSMiddleware(allowedOrigins string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("Access-Control-Allow-Origin", allowedOrigins)
			w.Header().Set("Access-Control-Allow-Methods", "GET, POST, PUT, DELETE, OPTIONS")
			w.Header().Set("Access-Control-Allow-Headers", "Content-Type, Authorization, X-Request-ID")
			w.Header().Set("Access-Control-Max-Age", "86400")

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}

// RequestTimeoutMiddleware sets a timeout for each request.
func RequestTimeoutMiddleware(timeout time.Duration) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx, cancel := context.WithTimeout(r.Context(), timeout)
			defer cancel()
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ContentTypeMiddleware sets the default content type to JSON.
func ContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodGet && r.Method != http.MethodDelete {
			if ct := r.Header.Get("Content-Type"); ct == "" {
				r.Header.Set("Content-Type", "application/json")
			}
		}
		next.ServeHTTP(w, r)
	})
}

// SecurityHeadersMiddleware adds basic security headers.
func SecurityHeadersMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("X-Content-Type-Options", "nosniff")
		w.Header().Set("X-Frame-Options", "DENY")
		w.Header().Set("X-XSS-Protection", "1; mode=block")
		w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
		next.ServeHTTP(w, r)
	})
}

// MetricsMiddleware wraps the next handler with metrics tracking.
func MetricsMiddleware(mc *model.MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			mc.RecordRequest(r.URL.Path, r.Method)
			
			sw := &statusWriter{ResponseWriter: w, statusCode: http.StatusOK}
			next.ServeHTTP(sw, r)
			
			duration := time.Since(start)
			mc.RecordLatency(duration)
			
			isSuccess := sw.statusCode >= 200 && sw.statusCode < 400
			mc.CompleteRequest(isSuccess)
			
			// For API paths, take a diagnostic snapshot
			if len(r.URL.Path) > 4 && r.URL.Path[:5] == "/api/" {
				go func() {
					snap := mc.ForceSnapshot()
					if snap.ActiveRequests > 100 {
						mc.SetMetric("high_traffic_alert", true)
					}
				}()
			}
		})
	}
}

// MetricsSnapshotMiddleware provides a way to read metrics snapshot via middleware.
func MetricsSnapshotMiddleware(mc *model.MetricsCollector) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			snap := mc.Snapshot()
			if snap.TotalRequests > 0 && snap.ActiveRequests > snap.TotalRequests {
				mc.SetMetric("inconsistent_snapshot", true)
			}
			next.ServeHTTP(w, r)
		})
	}
}

// MetricsSnapshot reads the current metrics snapshot.
func MetricsSnapshot(mc *model.MetricsCollector) model.MetricsSnapshot {
	return mc.Snapshot()
}
