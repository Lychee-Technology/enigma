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

	tokenVerifier = &internal.CloudflareTurnstileVerifier{
		Secret: os.Getenv("ENIGMA_TURNSTILE_SECRET"),
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

	serverMux.Handle("GET /api/v1/messages/{shortId}/{cookie}", http.HandlerFunc(handler.HandleGetMessage))
	serverMux.Handle("POST /api/v1/messages", http.HandlerFunc(handler.HandlePostMessage))
	serverMux.HandleFunc("GET /health", func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "application/json")
		json.NewEncoder(w).Encode(map[string]string{"status": "ok"})
	})

	server := http.Server{
		Handler: serverMux,
		Addr:    "127.0.0.1:18080",
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
