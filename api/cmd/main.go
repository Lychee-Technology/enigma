package main

import (
	"context"
	"encoding/json"
	"errors"
	"log"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/google/uuid"
	"lychee.technology/enigma/internal"
)

func main() {
	// Configure structured JSON logging
	logger := slog.New(slog.NewJSONHandler(os.Stdout, nil))
	slog.SetDefault(logger)

	// https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
	stage := os.Getenv("ENIGMA_STAGE")

	slog.Info("starting", "stage", stage)

	var dataSource *internal.OracleNoSqlEnigmaDataSource
	var err error
	var tokenVerifier internal.TokenVerifier

	dataSource, err = internal.NewOracleNoSqlEnigmaDataSource()

	if err != nil {
		log.Fatalf("Failed to create datasource, %v\n", err)
		return
	}

	if stage == "dev" {
		slog.Warn("running in dev mode — Turnstile verification is disabled")
		tokenVerifier = &internal.NoOpTokenVerifier{}
	} else {
		tokenVerifier = &internal.CloudflareTurnstileVerifier{
			Secret: os.Getenv("ENIGMA_TURNSTILE_SECRET"),
		}
	}

	repository := &internal.EnigmaMessageRepository{
		DataSource: dataSource,
	}

	defer repository.Close()

	handler := &internal.EnigmaHttpHandler{
		Repository:    repository,
		TokenVerifier: tokenVerifier,
	}

	slog.Info("starting Enigma API server")

	serverMux := http.NewServeMux()

	serverMux.Handle("DELETE /api/v1/messages/{shortId}/{cookie}", http.HandlerFunc(handler.HandleGetMessage))
	serverMux.Handle("POST /api/v1/messages", http.HandlerFunc(handler.HandlePostMessage))
	serverMux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	// Wrap all routes with request ID middleware.
	rootHandler := http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		reqID := r.Header.Get("X-Request-ID")
		if reqID == "" {
			reqID = uuid.New().String()
		}
		w.Header().Set("X-Request-ID", reqID)
		slog.Info("request", "method", r.Method, "path", r.URL.Path, "requestId", reqID)
		serverMux.ServeHTTP(w, r)
	})

	addr := os.Getenv("ENIGMA_LISTEN_ADDR")
	if addr == "" {
		addr = "127.0.0.1:18080"
	}

	server := http.Server{
		Handler: rootHandler,
		Addr:    addr,
	}

	slog.Info("listening", "addr", server.Addr)

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
		slog.Info("stopped serving new connections")
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}
	slog.Info("graceful shutdown complete")
}
