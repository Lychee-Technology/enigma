package main

import (
	"context"
	"crypto/tls"
	"errors"
	"log"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"lychee.technology/enigma/internal"
)

func main() {
	// https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
	stage := os.Getenv("ENIGMA_STAGE")

	log.Printf("Running in %q stage", stage)

	var dataSource *internal.OracleNoSqlEngimaDataSource
	var err error
	var tokenVerifier internal.TokenVerifier

	dataSource, err = internal.NewOracleNoSqlEngimaDataSource()

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

	handler := &internal.EngimaHttpHandler{
		Repository:    repository,
		TokenVerifier: tokenVerifier,
	}

	log.Println("Starting Enigma API server...")

	serverMux := http.NewServeMux()

	serverMux.Handle("GET /api/v1/messages/{shortId}/{cookie}", http.HandlerFunc(handler.HandleGetMessage))
	serverMux.Handle("POST /api/v1/messages", http.HandlerFunc(handler.HandlePostMessage))
	serverMux.Handle("OPTIONS /api/v1/{any...}", http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		responseWriter.WriteHeader(http.StatusOK)
	}))

	server := http.Server{
		Handler: serverMux,
		Addr:    "127.0.0.1:18080",
		TLSConfig: &tls.Config{
			// Set minimum TLS version
			MinVersion: tls.VersionTLS12,
		},
	}

	log.Printf("Listening to %v ...\n", server.Addr)

	go func() {
		if err := server.ListenAndServe(); !errors.Is(err, http.ErrServerClosed) {
			log.Fatalf("HTTP server error: %v", err)
		}
		log.Println("Stopped serving new connections.")
	}()

	sigChan := make(chan os.Signal, 1)
	signal.Notify(sigChan, syscall.SIGINT, syscall.SIGTERM)
	<-sigChan

	shutdownCtx, shutdownRelease := context.WithTimeout(context.Background(), 10*time.Second)
	defer shutdownRelease()

	if err := server.Shutdown(shutdownCtx); err != nil {
		log.Fatalf("HTTP shutdown error: %v", err)
	}
	log.Println("Graceful shutdown complete.")
}
