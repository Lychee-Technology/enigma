package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"regexp"
	"syscall"
	"time"

	"ruoshi.li/enigma/internal"
)


func main() {
	// https://dev.to/mokiat/proper-http-shutdown-in-go-3fji
	rootUrlParser := regexp.MustCompile(`^/([a-zA-Z0-9]{3,10})$`)

	dataSource, err := internal.NewEngimaDataSource()

	if err != nil {
		log.Fatalf("Failed to create datasource, %v\n", err)
	}

	service := &internal.EnigmaService{
		DataSource: dataSource,
	}
	defer service.Close()
	
	fmt.Println("Start")
	serverMux := http.NewServeMux()

	serverMux.Handle("GET /{shortId}", http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		shortId := rootUrlParser.FindStringSubmatch(request.URL.Path)

		if (len(shortId) != 2) {
			log.Printf("Invalid short id, path: %v\n", request.URL.Path)
			responseWriter.WriteHeader(http.StatusBadRequest)
			return
		}

		record, err := service.GetEnigmaRecord(shortId[1])

		if err != nil {
			http.NotFound(responseWriter, request)
			return
		}

		var redirectTo string
		if len(record.Cookie) == 0 {
			redirectTo = record.Content
		} else {
			redirectTo = fmt.Sprintf("/decrypt.html?id=%v", shortId)
		}
		http.Redirect(responseWriter, request, redirectTo, http.StatusTemporaryRedirect)
	}))

	serverMux.Handle("GET /api/v1/messages/{shortId}/{cookie}", http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		// TODO: read from db
		// if it has cookie, then render decrypt

		responseWriter.Write([]byte(fmt.Sprintf("OK, %v", request.URL.Path)))
	}))

	serverMux.Handle("POST /api/v1/messages", http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		// if no valid paramters, then render index page

		var requestObject internal.SaveMessageRequest

		json.NewDecoder(request.Body).Decode(&requestObject)
		
	 	resp, err := service.SaveMessage(&requestObject)

		if err != nil {
			log.Printf("Save message failed, %v\n", err)
		}

		json.NewEncoder(responseWriter).Encode(resp)
	}))

	serverMux.Handle("POST /api/v1/url", http.HandlerFunc(func(responseWriter http.ResponseWriter, request *http.Request) {
		// if no valid paramters, then render index page
		var requestObject internal.SaveUrlRequest

		json.NewDecoder(request.Body).Decode(&requestObject)
		
	 	resp, err := service.SaveUrl(&requestObject)

		if err != nil {
			log.Printf("Save URL failed, %v\n", err)
		}

		json.NewEncoder(responseWriter).Encode(resp)

	}))

	server := http.Server{
		Handler: serverMux,
		Addr:    "127.0.0.1:18080",
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
