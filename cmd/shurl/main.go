package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log"
	"net/http"
	"os"
	"time"

	"github.com/ubaas/ubaas/config"
	"github.com/ubaas/ubaas/model"
	"github.com/ubaas/ubaas/service"
	"github.com/ubaas/ubaas/store"
)

func main() {
	cfg := config.Default()
	cfg.Storage.URLFilePath("./shurl_data.json")
	cfg.Storage.LogFilePath("./shurl_access.log")

	us, err := store.NewURLStore(cfg)
	if err != nil {
		log.Printf("warning: URLStore init returned error: %v", err)
	}

	ls, err := store.NewAccessLogStore(cfg)
	if err != nil {
		log.Printf("warning: AccessLogStore init returned error: %v", err)
	}

	usvc, err := service.NewURLService(cfg, us)
	if err != nil {
		log.Fatalf("failed to create URLService: %v", err)
	}

	rsvc, err := service.NewRedirectService(us, ls)
	if err != nil {
		log.Fatalf("failed to create RedirectService: %v", err)
	}

	mux := http.NewServeMux()

	mux.HandleFunc("/health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]interface{}{
			"status": "ok",
			"time":   time.Now().UTC().Format(time.RFC3339),
		})
	})

	mux.HandleFunc("/api/create", func(w http.ResponseWriter, r *http.Request) {
		if r.Method != http.MethodPost {
			http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
			return
		}
		var req model.CreateReq
		if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		su, err := usvc.Create(ctx, &req)
		if err != nil {
			http.Error(w, err.Error(), http.StatusBadRequest)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(su)
	})

	mux.HandleFunc("/api/redirect", func(w http.ResponseWriter, r *http.Request) {
		code := r.URL.Query().Get("code")
		if code == "" {
			http.Error(w, "code required", http.StatusBadRequest)
			return
		}
		ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
		defer cancel()
		res, err := rsvc.HandleRedirect(ctx, &service.RedirectRequest{
			Code:      code,
			Timestamp: time.Now(),
		})
		if err != nil {
			http.Error(w, err.Error(), http.StatusNotFound)
			return
		}
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(res)
	})

	addr := fmt.Sprintf(":%s", os.Getenv("PORT"))
	if addr == ":" {
		addr = ":8080"
	}

	log.Printf("shurl server listening on %s", addr)
	if err := http.ListenAndServe(addr, mux); err != nil {
		log.Fatalf("server failed: %v", err)
	}
}
