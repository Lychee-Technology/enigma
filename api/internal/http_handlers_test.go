package internal_test

import (
	"bytes"
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"lychee.technology/enigma/internal"
)


type handlerFixture struct {
	ds      *mockDataSource
	handler *internal.EnigmaHttpHandler
}

func newFixture() *handlerFixture {
	mds := &mockDataSource{records: make([]*EnigmaRecord, 0)}
	repo := &EnigmaMessageRepository{DataSource: mds}
	h := &internal.EnigmaHttpHandler{
		Repository:    repo,
		TokenVerifier: &internal.NoOpTokenVerifier{},
	}
	return &handlerFixture{ds: mds, handler: h}
}

func newFixtureWithBadVerifier() *handlerFixture {
	mds := &mockDataSource{records: make([]*EnigmaRecord, 0)}
	repo := &EnigmaMessageRepository{DataSource: mds}

	// Inline bad verifier via mockDataSource abuse — use a separate type
	h := &internal.EnigmaHttpHandler{
		Repository:    repo,
		TokenVerifier: &badVerifier{},
	}
	return &handlerFixture{ds: mds, handler: h}
}

// badVerifier rejects every token.
type badVerifier struct{}

func (v *badVerifier) VerifyToken(_ context.Context, _ internal.ClientToken) error {
	return errors.New("invalid token")
}

// --- handlePostMessage tests ---

func TestHandlePostMessage_MissingAuth(t *testing.T) {
	f := newFixtureWithBadVerifier()
	body := `{"encryptedData":"abc","cookie":"cc","ttlHours":1}`
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Turnstile bad-token")
	rr := httptest.NewRecorder()

	f.handler.HandlePostMessage(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestHandlePostMessage_InvalidJSON(t *testing.T) {
	f := newFixture()
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", strings.NewReader("not-json"))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandlePostMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandlePostMessage_ContentTooLarge(t *testing.T) {
	f := newFixture()
	large := strings.Repeat("x", internal.MaxEncryptedDataLen+1)
	body, _ := json.Marshal(map[string]interface{}{
		"encryptedData": large,
		"cookie":        "cc",
		"ttlHours":      1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandlePostMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandlePostMessage_SaveFailure(t *testing.T) {
	f := newFixture()
	f.ds.saveErrs = []error{errors.New("db error")}
	body, _ := json.Marshal(map[string]interface{}{
		"encryptedData": "valid",
		"cookie":        "cc",
		"ttlHours":      1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandlePostMessage(rr, req)

	if rr.Code != http.StatusInternalServerError {
		t.Errorf("expected 500, got %d", rr.Code)
	}
}

func TestHandlePostMessage_Success(t *testing.T) {
	f := newFixture()
	body, _ := json.Marshal(map[string]interface{}{
		"encryptedData": "hello",
		"cookie":        "cc",
		"ttlHours":      1,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/messages", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandlePostMessage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if resp["shortId"] == "" {
		t.Error("expected non-empty shortId in response")
	}
}

// --- handleGetMessage tests ---

func TestHandleGetMessage_MissingAuth(t *testing.T) {
	f := newFixtureWithBadVerifier()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/abc/cookie", nil)
	req.Header.Set("Authorization", "Turnstile bad-token")
	rr := httptest.NewRecorder()

	f.handler.HandleGetMessage(rr, req)

	if rr.Code != http.StatusUnauthorized {
		t.Errorf("expected 401, got %d", rr.Code)
	}
}

func TestHandleGetMessage_ShortIdTooShort(t *testing.T) {
	f := newFixture()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/ab/ck", nil)
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandleGetMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetMessage_MalformedPath(t *testing.T) {
	f := newFixture()
	// Missing cookie segment
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/abc", nil)
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandleGetMessage(rr, req)

	if rr.Code != http.StatusBadRequest {
		t.Errorf("expected 400, got %d", rr.Code)
	}
}

func TestHandleGetMessage_NotFound(t *testing.T) {
	f := newFixture()
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/abc/wrongcookie", nil)
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandleGetMessage(rr, req)

	if rr.Code != http.StatusNotFound {
		t.Errorf("expected 404, got %d", rr.Code)
	}
}

func TestHandleGetMessage_Success(t *testing.T) {
	f := newFixture()
	// Pre-populate a record
	f.ds.records = []*EnigmaRecord{
		{SKey: "abc", ShortId: "abcdef", Cookie: "testcookie", Content: "secret"},
	}
	req := httptest.NewRequest(http.MethodGet, "/api/v1/messages/abcdef/testcookie", nil)
	req.Header.Set("Authorization", "Turnstile token")
	rr := httptest.NewRecorder()

	f.handler.HandleGetMessage(rr, req)

	if rr.Code != http.StatusOK {
		t.Errorf("expected 200, got %d", rr.Code)
	}
	var resp map[string]string
	if err := json.NewDecoder(rr.Body).Decode(&resp); err != nil {
		t.Fatalf("could not decode response: %v", err)
	}
	if resp["encryptedData"] != "secret" {
		t.Errorf("expected encryptedData=secret, got %q", resp["encryptedData"])
	}
}
