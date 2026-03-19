package internal

import (
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
)

type EnigmaHttpHandler struct {
	Repository    *EnigmaMessageRepository
	TokenVerifier TokenVerifier
}

func writeErrorJSON(w http.ResponseWriter, message string, status int) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(map[string]string{"error": message})
}

// tokenVerificationMiddleware creates a middleware that verifies the authentication token
func (handler *EnigmaHttpHandler) tokenVerificationMiddleware(next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		err := verifyToken(r, handler.TokenVerifier)
		if err != nil {
			slog.Warn("token verification failed", "error", err)
			writeErrorJSON(w, "invalid token", http.StatusUnauthorized)
			return
		}
		next(w, r)
	}
}

// handleGetMessage handles DELETE /api/v1/messages/{shortId}/{cookie}.
// Consuming (deleting) the message is an intentional side-effect of reading it.
func (handler *EnigmaHttpHandler) handleGetMessage(responseWriter http.ResponseWriter, request *http.Request) {
	shortId := request.PathValue("shortId")
	cookie := request.PathValue("cookie")

	if shortId == "" || cookie == "" {
		writeErrorJSON(responseWriter, "invalid path, expecting /api/v1/messages/{shortId}/{cookie}", http.StatusBadRequest)
		return
	}

	if len(shortId) < ShardKeyLen {
		writeErrorJSON(responseWriter, "shortId must be at least 3 letters", http.StatusBadRequest)
		return
	}

	record, err := handler.Repository.DeleteEnigmaRecord(shortId, cookie)
	if err != nil {
		slog.Warn("DeleteEnigmaRecord failed", "shortId", shortId, "error", err)
		writeErrorJSON(responseWriter, "not found", http.StatusNotFound)
		return
	}

	resp := &GetMessageResponse{
		EncryptedData: record.Content,
	}
	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(resp)
}

func (handler *EnigmaHttpHandler) handlePostMessage(responseWriter http.ResponseWriter, request *http.Request) {
	var requestObject SaveMessageRequest

	err := json.NewDecoder(request.Body).Decode(&requestObject)
	if err != nil {
		slog.Warn("failed to decode request body", "error", err)
		writeErrorJSON(responseWriter, "failed to decode request body", http.StatusBadRequest)
		return
	}

	resp, err := handler.Repository.SaveMessage(&requestObject)
	if err != nil {
		slog.Error("save message failed", "error", err)
		if errors.Is(err, ErrContentTooLarge) {
			writeErrorJSON(responseWriter, "content too large", http.StatusBadRequest)
		} else {
			writeErrorJSON(responseWriter, "internal server error", http.StatusInternalServerError)
		}
		return
	}

	responseWriter.Header().Set("Content-Type", "application/json")
	json.NewEncoder(responseWriter).Encode(resp)
}

func verifyToken(request *http.Request, tokenVerifier TokenVerifier) error {
	authHeader := request.Header.Get("Authorization")
	if authHeader == "" {
		return errors.New("missing Authorization header")
	}

	return tokenVerifier.VerifyToken(request.Context(), ClientToken{
		AuthorizationHeader: authHeader,
		IP:                  request.Header.Get("CF-Connecting-IP"),
	})
}

func (handler *EnigmaHttpHandler) HandleGetMessage(responseWriter http.ResponseWriter, request *http.Request) {
	handler.tokenVerificationMiddleware(handler.handleGetMessage).ServeHTTP(responseWriter, request)
}

func (handler *EnigmaHttpHandler) HandlePostMessage(responseWriter http.ResponseWriter, request *http.Request) {
	handler.tokenVerificationMiddleware(handler.handlePostMessage).ServeHTTP(responseWriter, request)
}
