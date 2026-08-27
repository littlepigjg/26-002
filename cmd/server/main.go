// Package main is the entry point for the UBAAS (User Behavior Analysis as a Service) application.
// It initializes all components, sets up the HTTP server, and handles graceful shutdown.
package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"syscall"
	"time"

	"github.com/ubaas/ubaas/internal/config"
	"github.com/ubaas/ubaas/internal/handler"
	"github.com/ubaas/ubaas/internal/middleware"
	"github.com/ubaas/ubaas/internal/service"
	"github.com/ubaas/ubaas/internal/store"
	"github.com/ubaas/ubaas/pkg/logger"
)

func main() {
	// Initialize configuration
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	// Initialize logger
	logLevel := logger.ParseLevel(cfg.Logging.Level)
	log := logger.New(os.Stdout, logLevel, "ubaas")
	log.Infof("Starting UBAAS server...")
	log.Infof("Configuration: %s", cfg.String())

	// Initialize store
	memStore := store.NewMemoryStore(log)
	defer memStore.Close()

	// Initialize services
	eventSvc := service.NewEventService(memStore, cfg, log)
	sessionSvc := service.NewSessionService(memStore, cfg, log)
	pathSvc := service.NewPathService(memStore, cfg, log)
	statsSvc := service.NewStatsService(memStore, cfg, log)
	convSvc := service.NewConversionService(memStore, cfg, log)
	dimSvc := service.NewDimensionService(memStore, cfg, log)
	exportSvc := service.NewExportService(memStore, cfg, log)
	scheduler := service.NewScheduler(memStore, log)

	// Start background tasks
	scheduler.Start()

	// Initialize router
	router := handler.NewAPIRouter(eventSvc, sessionSvc, pathSvc, statsSvc, convSvc, dimSvc, exportSvc)

	// Set up HTTP middleware chain
	var handler http.Handler = router.Handler()
	handler = middleware.SecurityHeadersMiddleware(handler)
	handler = middleware.CORSMiddleware("*")(handler)
	handler = middleware.ContentTypeMiddleware(handler)
	handler = middleware.RecoveryMiddleware(log)(handler)
	handler = middleware.LoggingMiddleware(log)(handler)

	// Serve static files for frontend
	webDir := filepath.Join("web")
	if _, err := os.Stat(webDir); err == nil {
		fs := http.FileServer(http.Dir(webDir))
		mux := http.NewServeMux()

		// Wrap the API handler
		apiHandler := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			// Check if it's an API or system request
			path := r.URL.Path
			if len(path) > 4 && path[:4] == "/api" {
				apiHandler.ServeHTTP(w, r)
				return
			}
			if path == "/health" || path == "/ready" {
				apiHandler.ServeHTTP(w, r)
				return
			}

			// Serve static files
			if path == "/" || path == "" {
				http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
		_ = mux
	}

	// Create HTTP server
	addr := cfg.Server.Addr()
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	// Start server in a goroutine
	go func() {
		log.Infof("HTTP server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	// Wait for interrupt signal
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	// Graceful shutdown
	shutdownCtx, cancel := context.WithTimeout(context.Background(),
		time.Duration(cfg.Server.ShutdownTimeout)*time.Second)
	defer cancel()

	// Stop background services first
	scheduler.Stop()
	eventSvc.Stop()

	// Shutdown HTTP server
	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Errorf("HTTP server shutdown error: %v", err)
	}

	log.Info("Server stopped successfully")
	log.Infof("Uptime: %v", getUptime())
}

// getUptime returns server uptime as a string.
func getUptime() string {
	// This would use a startup time variable in production
	return "completed"
}


