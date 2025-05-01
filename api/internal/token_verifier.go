package internal

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log"
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

	if token.IP == "" {
		values.Set("remoteip", token.IP)
	}

	log.Printf("Creating Values for Turnstile verification, turnstile response: '%s...', idempotency key: '%s', remote IP: '%s'\n",
		turnstileResponse[0:min(10, len(turnstileResponse))],
		idempotencyKey,
		token.IP)

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
		log.Printf("Failed to create values for Turnstile verification: %v", err)
		return err
	}

	var resp *http.Response
	backoffs := []time.Duration{100 * time.Millisecond, 200 * time.Millisecond}
	for i, delay := range backoffs {
		log.Printf("Turnstile verification attempt %d", i+1)
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
				log.Printf("Turnstile verification failed: %v", err)
				if isRetriable {
					log.Printf("Retrying Turnstile verification...")
				} else {
					return err
				}
			}
		}

		if err != nil {
			log.Printf("Turnstile verification error: %v", err)
		} else {
			log.Printf("Turnstile verification successful")
			return nil
		}
		if i < len(backoffs)-1 {
			log.Printf("Retrying in %v...", delay)
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
		log.Printf("Failed to decode Turnstile response: %v", err)
		return true, err
	}

	if result.Success {
		return false, nil
	}

	if len(result.ErrorCodes) == 1 && (result.ErrorCodes[0] == "internal-error" || result.ErrorCodes[0] == "timeout-or-duplicate") {
		log.Printf("Turnstile internal error, retrying...")
		return true, fmt.Errorf("retriable turnstile verification error, error codes: %v", result.ErrorCodes)
	} else {
		log.Printf("Turnstile verification failed: %v", result.ErrorCodes)
		return false, fmt.Errorf("turnstile verification failed, error codes: %v", result.ErrorCodes)
	}
}
