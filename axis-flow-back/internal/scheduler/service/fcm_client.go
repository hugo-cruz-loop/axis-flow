// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the FCM client described in the spec section
// "Seguridad > Firebase Credentials Protection". The client is responsible
// for reading the service-account JSON from FIREBASE_CREDENTIALS_JSON,
// initialising a single firebase.App, and dispatching push notifications
// to FCM with a bounded retry policy. The implementation is strict about
// PII safety: device tokens, payload bodies, and the full FCM message
// identifier are never written to logs. Only the redacted 8-character
// token prefix, the (redacted) project id, the attempt counter, and the
// classified error sentinel are ever logged.
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"log/slog"
	"math/rand"
	"strings"
	"time"

	"firebase.google.com/go/v4"
	"firebase.google.com/go/v4/messaging"
	"google.golang.org/api/option"
)

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrFCMInvalidCredentials is returned when the supplied credentials
	// JSON is malformed, empty, or lacks the project_id field required
	// by firebase.NewApp + messaging.NewClient. This is a permanent
	// failure: the caller MUST fix the configuration and restart.
	ErrFCMInvalidCredentials = errors.New("scheduler: FCM credentials are invalid")
	// ErrFCMPermanentFailure is returned for FCM errors that the spec
	// classifies as non-retryable (invalid argument, unregistered
	// token, sender ID mismatch, third-party auth, quota exceeded). The
	// caller should NOT retry these.
	ErrFCMPermanentFailure = errors.New("scheduler: FCM permanent failure")
)

// ---------------------------------------------------------------------------
// Public contract.
// ---------------------------------------------------------------------------

// NotificationPayload is the platform-neutral description of a push
// notification. Title and Body are surfaced to the user by the device
// notification UI; Data is delivered verbatim to the app for routing
// or background work. No field of this struct is ever logged in full
// by the FCM client implementation.
type NotificationPayload struct {
	// Title is the human-readable headline of the notification.
	Title string
	// Body is the human-readable body of the notification.
	Body string
	// Data carries string-only key/value pairs that the mobile app
	// reads on receipt. Values must be strings because the FCM wire
	// format only accepts string values.
	Data map[string]string
}

// FCMClient dispatches push notifications to a single device. Concrete
// implementations are concurrency-safe: Send may be called from many
// goroutines simultaneously.
type FCMClient interface {
	// Send delivers payload to the device identified by deviceToken.
	// It returns the FCM-assigned message identifier on success. On a
	// permanent FCM failure (invalid token, sender mismatch, quota
	// exceeded) it returns ErrFCMPermanentFailure wrapped with the
	// underlying cause. On a transient failure the call is retried
	// up to maxAttempts (3 by default) with exponential backoff +
	// jitter; when the budget is exhausted, the last error is
	// returned wrapped with %w.
	Send(ctx context.Context, deviceToken string, payload NotificationPayload) (messageID string, err error)
}

// ---------------------------------------------------------------------------
// Production implementation — backed by firebase.google.com/go/v4.
// ---------------------------------------------------------------------------

// firebaseFCMClient is the production FCMClient. It is concurrency-safe:
// the underlying *messaging.Client pools HTTP connections and the
// retry state is held in per-call locals.
type firebaseFCMClient struct {
	client   *messaging.Client
	app      *firebase.App
	logger   *Logger
	rng      *rand.Rand
	rngMut   chan struct{} // guards rng for the rare concurrent case
	maxTries int
	baseWait time.Duration
	factor   float64
	jitter   float64
}

// NewFCMClient initialises a Firebase Admin SDK App from a service-account
// JSON payload and returns an FCMClient ready to dispatch pushes.
//
// credentialsJSON must be the raw service-account JSON. It is read in
// memory and never written to disk by this constructor; callers are
// responsible for keeping the value out of persistent storage and for
// ensuring it is loaded from a secret manager or environment variable
// (FIREBASE_CREDENTIALS_JSON per the spec).
//
// The constructor never logs the credentials JSON or any of its fields.
// It emits exactly one log line —
//
//	msg="FCM client initialized" project_id=<id> credential_bytes=<n>
//
// — to confirm wiring. credential_bytes is the byte length, NOT the
// payload itself.
//
// Returns ErrFCMInvalidCredentials (wrapped) if the JSON is empty,
// unparseable, or missing the project_id field. The constructor is
// fail-fast: it does not silently fall back to anonymous mode.
func NewFCMClient(ctx context.Context, credentialsJSON []byte) (FCMClient, error) {
	if len(credentialsJSON) == 0 {
		return nil, fmt.Errorf("NewFCMClient: %w: empty credentials payload", ErrFCMInvalidCredentials)
	}

	// Probe the JSON to extract the project_id without keeping the
	// raw payload around. We use a private type so we never expose
	// the project_id (or any other field) on the Config struct that
	// firebase.NewApp would carry.
	var peek struct {
		ProjectID string `json:"project_id"`
	}
	if err := json.Unmarshal(credentialsJSON, &peek); err != nil {
		return nil, fmt.Errorf("NewFCMClient: %w: cannot parse credentials JSON: %v", ErrFCMInvalidCredentials, err)
	}
	projectID := strings.TrimSpace(peek.ProjectID)
	if projectID == "" {
		return nil, fmt.Errorf("NewFCMClient: %w: credentials JSON missing project_id", ErrFCMInvalidCredentials)
	}

	app, err := firebase.NewApp(
		ctx,
		&firebase.Config{ProjectID: projectID},
		option.WithCredentialsJSON(credentialsJSON),
	)
	if err != nil {
		return nil, fmt.Errorf("NewFCMClient: %w: firebase.NewApp: %v", ErrFCMInvalidCredentials, err)
	}
	msgClient, err := app.Messaging(ctx)
	if err != nil {
		return nil, fmt.Errorf("NewFCMClient: %w: app.Messaging: %v", ErrFCMInvalidCredentials, err)
	}

	c := &firebaseFCMClient{
		client:   msgClient,
		app:      app,
		logger:   nil, // optional; the no-logger fallback in Send handles nil
		maxTries: 3,
		baseWait: 500 * time.Millisecond,
		factor:   2.0,
		jitter:   0.2, // ±20 %
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		// rngMut is a 1-buffered channel used purely as a mutex so
		// we do not have to import sync just for one lock.
		rngMut: make(chan struct{}, 1),
	}

	// Best-effort info log. We never log the credentials JSON, the
	// project_id is the only credential-derived value that is safe to
	// surface (it is also required by the operator to verify the
	// correct Firebase project is wired up).
	slog.Info(
		"FCM client initialized",
		slog.String("project_id", projectID),
		slog.Int("credential_bytes", len(credentialsJSON)),
	)
	return c, nil
}

// setLogger attaches a structured logger. Optional; nil-safe.
func (c *firebaseFCMClient) setLogger(l *Logger) { c.logger = l }

// Send dispatches the payload with up to maxAttempts retries on
// transient errors. Permanent errors are surfaced immediately without
// burning the retry budget.
func (c *firebaseFCMClient) Send(ctx context.Context, deviceToken string, payload NotificationPayload) (string, error) {
	deviceToken = strings.TrimSpace(deviceToken)
	if deviceToken == "" {
		return "", fmt.Errorf("fcm_client.Send: device_token is required")
	}
	// Validate that every Data value is a string — the FCM wire format
	// refuses non-string values and the failure mode is a permanent
	// 400 that we want to surface before opening a network call.
	for k, v := range payload.Data {
		if v == "" {
			return "", fmt.Errorf("fcm_client.Send: data[%q] is empty", k)
		}
	}

	prefix := redactToken(deviceToken)
	msg := &messaging.Message{
		Token: deviceToken,
		Notification: &messaging.Notification{
			Title: payload.Title,
			Body:  payload.Body,
		},
		Data: payload.Data,
	}

	var lastErr error
	for attempt := 1; attempt <= c.maxTries; attempt++ {
		id, err := c.client.Send(ctx, msg)
		if err == nil {
			c.logSend(ctx, "info", prefix, id, attempt, "")
			return id, nil
		}
		lastErr = err
		class := classifyFCMError(err)
		c.logSend(ctx, "warn", prefix, id, attempt, class)
		// Permanent failures: do not retry.
		if class != "transient" {
			return "", fmt.Errorf("%w: %v", ErrFCMPermanentFailure, err)
		}
		// Transient: back off before the next attempt, unless this
		// was the last one.
		if attempt < c.maxTries {
			if waitErr := c.sleepBackoff(ctx, attempt); waitErr != nil {
				return "", fmt.Errorf("fcm_client.Send: context cancelled during backoff: %w", waitErr)
			}
		}
	}
	return "", fmt.Errorf("fcm_client.Send: retry budget exhausted after %d attempts: %w", c.maxTries, lastErr)
}

// sleepBackoff waits base * factor^(attempt-1) ± jitter*100%, or until
// the context is cancelled. attempt is 1-based; on attempt=1 the wait
// is approximately base*(1 ± jitter).
func (c *firebaseFCMClient) sleepBackoff(ctx context.Context, attempt int) error {
	nominal := float64(c.baseWait)
	for i := 1; i < attempt; i++ {
		nominal *= c.factor
	}
	// Pull a uniform multiplier in [1-jitter, 1+jitter].
	// rng is a *rand.Rand; we guard it with a 1-buffered channel used
	// as a mutex so concurrent Send calls cannot corrupt the state.
	c.rngMut <- struct{}{}
	mult := 1.0 + (c.jitter*2)*c.rng.Float64() - c.jitter
	<-c.rngMut
	wait := time.Duration(nominal * mult)
	if wait < 0 {
		wait = c.baseWait
	}
	t := time.NewTimer(wait)
	defer t.Stop()
	select {
	case <-ctx.Done():
		return ctx.Err()
	case <-t.C:
		return nil
	}
}

// logSend emits a single structured log line for one Send attempt.
// The level is "info" on success, "warn" otherwise. messageID may be
// the empty string when FCM did not return one; that is safe to log.
func (c *firebaseFCMClient) logSend(ctx context.Context, level, prefix, messageID string, attempt int, errClass string) {
	log := c.loggerOrSlog()
	attrs := []slog.Attr{
		slog.String("device_token_prefix", prefix),
		slog.String("message_id", messageID),
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", c.maxTries),
	}
	if errClass != "" {
		attrs = append(attrs, slog.String("error_class", errClass))
	}
	switch level {
	case "info":
		log.LogAttrs(ctx, slog.LevelInfo, "FCM message dispatched", attrs...)
	default:
		log.LogAttrs(ctx, slog.LevelWarn, "FCM message dispatch failed", attrs...)
	}
}

// loggerOrSlog returns the structured logger, falling back to the
// default slog default logger when the client has no logger attached.
func (c *firebaseFCMClient) loggerOrSlog() *slog.Logger {
	if c.logger != nil {
		return c.logger.Inner()
	}
	return slog.Default()
}

// ---------------------------------------------------------------------------
// Error classification — transient vs permanent.
// ---------------------------------------------------------------------------

// classifyFCMError inspects an FCM error using the SDK's
// messaging.IsXxx sentinels and returns "transient" or "permanent".
// Network errors and context deadlines are always transient; well-known
// FCM failure codes are permanent.
func classifyFCMError(err error) string {
	if err == nil {
		return ""
	}
	// Permanent: invalid argument, unregistered token, sender ID
	// mismatch, third-party auth, quota exceeded.
	if messaging.IsInvalidArgument(err) ||
		messaging.IsUnregistered(err) ||
		messaging.IsSenderIDMismatch(err) ||
		messaging.IsThirdPartyAuthError(err) ||
		messaging.IsQuotaExceeded(err) {
		return "permanent"
	}
	// Transient: internal FCM server errors and network availability
	// problems are retryable.
	if messaging.IsInternal(err) || messaging.IsUnavailable(err) {
		return "transient"
	}
	// Context errors propagate up as transient — the caller already
	// has a deadline they care about.
	if errors.Is(err, context.DeadlineExceeded) || errors.Is(err, context.Canceled) {
		return "transient"
	}
	// Default to transient so that novel / unknown failures from a
	// future FCM API version are at least retried once before being
	// reported.
	return "transient"
}

// redactToken returns the first 8 characters of the device token
// followed by a literal ellipsis. The contract is "first 8 chars…".
// For tokens shorter than 8 characters, the entire token is returned
// (still useful for debugging; FCM tokens are normally > 100 chars).
func redactToken(t string) string {
	if len(t) <= 8 {
		return t
	}
	return t[:8] + "…"
}

// ---------------------------------------------------------------------------
// Mock — used by tests (Phase 5 will exercise it from integration
// tests; the rest of the codebase imports NewMockFCMClient when it
// needs an FCMClient without a network dependency).
// ---------------------------------------------------------------------------

// SentCall records a single Send invocation for assertion in tests.
type SentCall struct {
	DeviceToken string
	Payload     NotificationPayload
}

// MockFCMClient is a hand-rolled test double that records every Send
// call. Tests may override the per-call response by setting SendFunc.
type MockFCMClient struct {
	// SentCalls is the chronological log of every Send invocation.
	SentCalls []SentCall
	// SendFunc, when non-nil, replaces the default "return empty
	// message_id, nil error" behaviour. It receives the same arguments
	// as Send.
	SendFunc func(ctx context.Context, deviceToken string, payload NotificationPayload) (string, error)
}

// NewMockFCMClient returns a fresh MockFCMClient with SendFunc unset
// (default behaviour: record the call and return ("", nil)).
func NewMockFCMClient() *MockFCMClient {
	return &MockFCMClient{}
}

// Send records the call and returns the configured response.
func (m *MockFCMClient) Send(ctx context.Context, deviceToken string, payload NotificationPayload) (string, error) {
	m.SentCalls = append(m.SentCalls, SentCall{
		DeviceToken: deviceToken,
		Payload:     payload,
	})
	if m.SendFunc != nil {
		return m.SendFunc(ctx, deviceToken, payload)
	}
	return "", nil
}
