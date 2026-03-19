package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"net/url"
	"strings"
	"time"

	"github.com/google/uuid"
)

type ClientToken struct {
	AuthorizationHeader string
	IP                  string
}

func (token *ClientToken) toValues(secret string) (*url.Values, error) {
	// Return error if token is missing
	if token.AuthorizationHeader == "" {
		return nil, errors.New("missing token")
	}

	// Expecting "Turnstile <token>"
	parts := strings.SplitN(token.AuthorizationHeader, " ", 2)
	if len(parts) != 2 || parts[0] != "Turnstile" {
		return nil, errors.New("invalid Authorization header format")
	}

	idempotencyKey := uuid.New().String()
	turnstileResponse := parts[1]

	values := url.Values{
		"secret":          {secret},
		"response":        {turnstileResponse},
		"idempotency_key": {idempotencyKey},
	}

	if token.IP != "" {
		values.Set("remoteip", token.IP)
	}

	slog.Info("creating turnstile verification request",
		"responsePrefix", turnstileResponse[0:min(10, len(turnstileResponse))],
		"idempotencyKey", idempotencyKey,
		"remoteIP", token.IP)

	return &values, nil
}

type TokenVerifier interface {
	// VerifyToken verifies the token using the provided secret.
	VerifyToken(context context.Context, token ClientToken) error
}

type CloudflareTurnstileVerifier struct {
	Secret string
}

type NoOpTokenVerifier struct {
}

func (verfier *NoOpTokenVerifier) VerifyToken(context context.Context, token ClientToken) error {
	// No verification needed, always return nil
	return nil
}

// verifyTurnstileToken sends the token and secret to Cloudflare Turnstile for verification.
func (verfier *CloudflareTurnstileVerifier) VerifyToken(context context.Context, token ClientToken) error {
	// Prepare form data
	data, err := token.toValues(verfier.Secret)

	if err != nil {
		slog.Warn("failed to create turnstile values", "error", err)
		return err
	}

	var resp *http.Response
	backoffs := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}
	for i, delay := range backoffs {
		slog.Info("turnstile verification attempt", "attempt", i+1)
		req, err := http.NewRequestWithContext(
			context,
			http.MethodPost,
			"https://challenges.cloudflare.com/turnstile/v0/siteverify",
			strings.NewReader(data.Encode()))

		if err != nil {
			return err
		}

		req.Header.Set("Content-Type", "application/x-www-form-urlencoded")
		client := &http.Client{Timeout: 5 * time.Second}
		resp, err = client.Do(req)
		if err == nil {
			isRetriable, err := checkVerificationResult(resp)
			if err != nil {
				slog.Warn("turnstile verification failed", "error", err, "retriable", isRetriable)
				if !isRetriable {
					return err
				}
			}
		}

		if err != nil {
			slog.Warn("turnstile verification error", "error", err)
		} else {
			slog.Info("turnstile verification successful")
			return nil
		}
		if i < len(backoffs)-1 {
			slog.Info("retrying turnstile verification", "delay", delay)
			time.Sleep(delay)
		}
	}

	return err
}

func isRetriableError(statusCode int) bool {
	if statusCode >= 500 && statusCode < 600 {
		return true
	}
	return statusCode == http.StatusTooManyRequests || statusCode == http.StatusRequestTimeout || statusCode == http.StatusTooEarly
}

func checkVerificationResult(resp *http.Response) (bool, error) {
	defer resp.Body.Close()

	if resp.StatusCode < http.StatusOK || resp.StatusCode >= http.StatusMultipleChoices {
		return isRetriableError(resp.StatusCode), errors.New("turnstile verification http request failed")
	}

	var result struct {
		Success     bool     `json:"success"`
		ChallengeTS string   `json:"challenge_ts"`
		Hostname    string   `json:"hostname"`
		ErrorCodes  []string `json:"error-codes"`
	}

	err := json.NewDecoder(resp.Body).Decode(&result)
	if err != nil {
		slog.Warn("failed to decode turnstile response", "error", err)
		return true, err
	}

	if result.Success {
		return false, nil
	}

	if len(result.ErrorCodes) == 1 && (result.ErrorCodes[0] == "internal-error" || result.ErrorCodes[0] == "timeout-or-duplicate") {
		slog.Warn("turnstile internal error, will retry", "errorCodes", result.ErrorCodes)
		return true, fmt.Errorf("retriable turnstile verification error, error codes: %v", result.ErrorCodes)
	} else {
		slog.Warn("turnstile verification failed", "errorCodes", result.ErrorCodes)
		return false, fmt.Errorf("turnstile verification failed, error codes: %v", result.ErrorCodes)
	}
}
