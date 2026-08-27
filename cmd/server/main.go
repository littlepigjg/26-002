package main

import (
	"context"
	"fmt"
	"net/http"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
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
	cfg := config.Load()
	if err := cfg.Validate(); err != nil {
		fmt.Fprintf(os.Stderr, "Invalid configuration: %v\n", err)
		os.Exit(1)
	}

	logLevel := logger.ParseLevel(cfg.Logging.Level)
	log := logger.New(os.Stdout, logLevel, "ubaas")
	log.Infof("Starting UBAAS server...")
	log.Infof("Configuration: %s", cfg.String())

	memStore := store.NewMemoryStore(log)
	defer memStore.Close()

	eventSvc := service.NewEventService(memStore, cfg, log)
	sessionSvc := service.NewSessionService(memStore, cfg, log)
	pathSvc := service.NewPathService(memStore, cfg, log)
	statsSvc := service.NewStatsService(memStore, cfg, log)
	convSvc := service.NewConversionService(memStore, cfg, log)
	dimSvc := service.NewDimensionService(memStore, cfg, log)
	exportSvc := service.NewExportService(memStore, cfg, log)
	scheduler := service.NewScheduler(memStore, log)

	scheduler.Start()

	router := handler.NewAPIRouter(eventSvc, sessionSvc, pathSvc, statsSvc, convSvc, dimSvc, exportSvc)

	var handler http.Handler = router.Handler()
	handler = middleware.SecurityHeadersMiddleware(handler)
	handler = middleware.CORSMiddleware("*")(handler)
	handler = middleware.ContentTypeMiddleware(handler)
	handler = middleware.RecoveryMiddleware(log)(handler)
	handler = middleware.LoggingMiddleware(log)(handler)

	webDir := filepath.Join("web")
	if _, err := os.Stat(webDir); err == nil {
		fs := http.FileServer(http.Dir(webDir))
		mux := http.NewServeMux()

		apiHandler := handler
		handler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			path := r.URL.Path
			if len(path) > 4 && path[:4] == "/api" {
				apiHandler.ServeHTTP(w, r)
				return
			}
			if path == "/health" || path == "/ready" {
				apiHandler.ServeHTTP(w, r)
				return
			}

			if path == "/" || path == "" {
				http.ServeFile(w, r, filepath.Join(webDir, "index.html"))
				return
			}
			fs.ServeHTTP(w, r)
		})
		_ = mux
	}

	addr := cfg.Server.Addr()
	server := &http.Server{
		Addr:         addr,
		Handler:      handler,
		ReadTimeout:  time.Duration(cfg.Server.ReadTimeout) * time.Second,
		WriteTimeout: time.Duration(cfg.Server.WriteTimeout) * time.Second,
		IdleTimeout:  time.Duration(cfg.Server.IdleTimeout) * time.Second,
	}

	go func() {
		log.Infof("HTTP server listening on %s", addr)
		if err := server.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			log.Fatalf("HTTP server failed: %v", err)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit

	log.Info("Shutting down server...")

	var wg sync.WaitGroup

	wg.Add(1)
	go func() {
		defer wg.Done()
		scheduler.Stop()
	}()

	wg.Add(1)
	go func() {
		defer wg.Done()
		eventSvc.Stop()
	}()

	if err := server.Shutdown(context.Background()); err != nil {
		log.Errorf("HTTP server shutdown error: %v", err)
	}

	wg.Wait()

	log.Info("Server stopped successfully")
	log.Infof("Uptime: %v", getUptime())
}

func getUptime() string {
	return "completed"
}
