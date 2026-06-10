// Package turnstile provides Cloudflare Turnstile captcha verification.
package turnstile

import (
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/url"
	"strings"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
)

const (
	defaultEndpoint = "https://challenges.cloudflare.com/turnstile/v0/siteverify"
	verifyTimeout   = 3 * time.Second
)

// TurnstileClient is the port for Cloudflare Turnstile captcha validation.
type TurnstileClient interface {
	// Verify validates a Turnstile token submitted by a client.
	// Returns bolsatrabajo.ErrCaptchaFail when the token is invalid.
	Verify(ctx context.Context, token, remoteIP string) error
}

// MockTurnstile always passes — used when TURNSTILE_ENABLED=false.
type MockTurnstile struct{}

// Verify always returns nil.
func (MockTurnstile) Verify(_ context.Context, _, _ string) error { return nil }

// HTTPTurnstile calls the real Cloudflare siteverify endpoint.
type HTTPTurnstile struct {
	endpoint string
	// secret is never logged — keep unexported.
	secret string
	client *http.Client
}

// Verify POSTs to the Turnstile siteverify endpoint and returns an error
// when success is false. The secret is sent in the POST body and is never
// included in logs or error messages.
func (h *HTTPTurnstile) Verify(ctx context.Context, token, remoteIP string) error {
	ctxTimeout, cancel := context.WithTimeout(ctx, verifyTimeout)
	defer cancel()

	body := url.Values{
		"secret":   {h.secret},
		"response": {token},
		"remoteip": {remoteIP},
	}

	req, err := http.NewRequestWithContext(ctxTimeout, http.MethodPost, h.endpoint, strings.NewReader(body.Encode()))
	if err != nil {
		return fmt.Errorf("turnstile: build request: %w", err)
	}
	req.Header.Set("Content-Type", "application/x-www-form-urlencoded")

	resp, err := h.client.Do(req)
	if err != nil {
		return fmt.Errorf("turnstile: request failed: %w", err)
	}
	defer resp.Body.Close()

	var result struct {
		Success bool `json:"success"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&result); err != nil {
		return fmt.Errorf("turnstile: decode response: %w", err)
	}

	if !result.Success {
		return bolsatrabajo.ErrCaptchaFail
	}
	return nil
}

// New returns a TurnstileClient. When enabled is false it returns MockTurnstile.
// endpoint defaults to the Cloudflare siteverify URL when empty.
func New(enabled bool, secret, endpoint string) TurnstileClient {
	if !enabled {
		return MockTurnstile{}
	}
	if endpoint == "" {
		endpoint = defaultEndpoint
	}
	return &HTTPTurnstile{
		endpoint: endpoint,
		secret:   secret,
		client:   &http.Client{Timeout: verifyTimeout},
	}
}
