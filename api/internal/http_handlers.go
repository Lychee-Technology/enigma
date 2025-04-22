package internal

import (
	"encoding/json"
	"errors"
	"log"
	"net/http"
	"strings"
)

type EngimaHttpHandler struct {
	Repository    *EnigmaMessageRepository
	TokenVerifier TokenVerifier
}

// TokenVerificationMiddleware creates a middleware that verifies the authentication token
func (handler *EngimaHttpHandler) tokenVerificationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := verifyToken(r, handler.TokenVerifier)
		if err != nil {
			log.Printf("Token verification failed: %v", err)
			http.Error(w, "Invalid token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

func (handler *EngimaHttpHandler) handleGetMessage(responseWriter http.ResponseWriter, request *http.Request) {
	path := request.URL.Path
	const prefix = "/api/v1/messages/"
	if !strings.HasPrefix(path, prefix) {
		http.NotFound(responseWriter, request)
		return
	}
	parts := strings.Split(strings.TrimPrefix(path, prefix), "/")
	if len(parts) < 2 {
		http.Error(responseWriter, "invalid path, expecting /api/v1/messages/{shortId}/{cookie}", http.StatusBadRequest)
		return
	}
	shortId := parts[0]
	cookie := parts[1]

	// Validate shortId length
	if len(shortId) < 3 {
		http.Error(responseWriter, "shortId must be at least 3 letters", http.StatusBadRequest)
		return
	}

	record, err := handler.Repository.DeleteEnigmaRecord(shortId, cookie)
	if err != nil {
		log.Printf("DeleteEnigmaRecord failed, %v\n", err)
		responseWriter.WriteHeader(http.StatusNotFound)
		responseWriter.Write([]byte("Not found"))
		return
	}

	resp := &GetMessageResponse{
		EncryptedData: record.Content,
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(resp)
}

func (handler *EngimaHttpHandler) handlePostMessage(responseWriter http.ResponseWriter, request *http.Request) {
	var requestObject SaveMessageRequest

	err := json.NewDecoder(request.Body).Decode(&requestObject)
	if err != nil {
		log.Printf("Failed to decode request body, %v\n", err)
		responseWriter.WriteHeader(http.StatusBadRequest)
		responseWriter.Write([]byte("Failed to decode request body."))
		return
	}

	resp, err := handler.Repository.SaveMessage(&requestObject)

	if err != nil {
		log.Printf("Save message failed, %v\n", err)
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(resp)
}

func verifyToken(request *http.Request, tokenVerifier TokenVerifier) error {
	authToken := request.Header.Get("Authorization")

	if len(authToken) == 0 {
		return errors.New("missing Authorization header")
	}

	return tokenVerifier.VerifyToken(request.Context(), authToken)
}

func (handler *EngimaHttpHandler) HandleGetMessage(responseWriter http.ResponseWriter, request *http.Request) {
	handler.tokenVerificationMiddleware(handler.handleGetMessage).ServeHTTP(responseWriter, request)
}

func (handler *EngimaHttpHandler) HandlePostMessage(responseWriter http.ResponseWriter, request *http.Request) {
	handler.tokenVerificationMiddleware(handler.handlePostMessage).ServeHTTP(responseWriter, request)
}