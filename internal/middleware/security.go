package middleware

import (
	"fmt"
	"net/http"
	"strings"
	"time"

	"github.com/ubaas/ubaas/pkg/logger"
)

// SecurityHeaders adds security-related HTTP headers to all responses.
func SecurityHeaders(log *logger.Logger) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			w.Header().Set("X-Content-Type-Options", "nosniff")
			w.Header().Set("X-Frame-Options", "DENY")
			w.Header().Set("X-XSS-Protection", "1; mode=block")
			w.Header().Set("Referrer-Policy", "strict-origin-when-cross-origin")
			w.Header().Set("Content-Security-Policy", "default-src 'self'")
			w.Header().Set("Strict-Transport-Security", "max-age=31536000; includeSubDomains")
			next.ServeHTTP(w, r)
		})
	}
}

// ValidateContentTypeMiddleware validates Content-Type for write operations.
func ValidateContentTypeMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "POST" || r.Method == "PUT" || r.Method == "PATCH" {
			contentType := r.Header.Get("Content-Type")
			if contentType != "" && !strings.Contains(contentType, "application/json") {
				http.Error(w, `{"code":400,"message":"Content-Type must be application/json"}`, http.StatusBadRequest)
				return
			}
		}
		next.ServeHTTP(w, r)
	})
}

// MethodNotAllowedHandler returns a handler for unsupported HTTP methods.
func MethodNotAllowedHandler(w http.ResponseWriter, allowedMethods ...string) {
	w.Header().Set("Allow", strings.Join(allowedMethods, ", "))
	http.Error(w, `{"code":405,"message":"Method not allowed"}`, http.StatusMethodNotAllowed)
}

// CORSOptions configures CORS behavior.
type CORSOptions struct {
	AllowedOrigins   []string
	AllowedMethods   []string
	AllowedHeaders   []string
	ExposedHeaders   []string
	AllowCredentials bool
	MaxAge           time.Duration
}

// DefaultCORSOptions returns default CORS options.
func DefaultCORSOptions() CORSOptions {
	return CORSOptions{
		AllowedOrigins:   []string{"*"},
		AllowedMethods:   []string{"GET", "POST", "PUT", "DELETE", "OPTIONS"},
		AllowedHeaders:   []string{"Content-Type", "Authorization", "X-Request-ID"},
		ExposedHeaders:   []string{"Content-Length", "X-Request-ID"},
		AllowCredentials: false,
		MaxAge:           12 * time.Hour,
	}
}

// CORSWithOptions creates a CORS middleware with custom options.
func CORSWithOptions(opts CORSOptions) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			origin := r.Header.Get("Origin")

			allowed := false
			for _, o := range opts.AllowedOrigins {
				if o == "*" || o == origin {
					allowed = true
					break
				}
			}

			if allowed {
				w.Header().Set("Access-Control-Allow-Origin", origin)
				w.Header().Set("Access-Control-Allow-Methods", strings.Join(opts.AllowedMethods, ", "))
				w.Header().Set("Access-Control-Allow-Headers", strings.Join(opts.AllowedHeaders, ", "))
				w.Header().Set("Access-Control-Expose-Headers", strings.Join(opts.ExposedHeaders, ", "))
				if opts.AllowCredentials {
					w.Header().Set("Access-Control-Allow-Credentials", "true")
				}
				if opts.MaxAge > 0 {
					w.Header().Set("Access-Control-Max-Age", fmt.Sprintf("%.0f", opts.MaxAge.Seconds()))
				}
			}

			if r.Method == http.MethodOptions {
				w.WriteHeader(http.StatusNoContent)
				return
			}

			next.ServeHTTP(w, r)
		})
	}
}
