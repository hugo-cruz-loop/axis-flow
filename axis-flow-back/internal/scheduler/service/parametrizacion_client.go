// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the Parametrizacion Service HTTP client used by
// the auto-deactivation job (Phase 3, task 3.3) to read the per-company
// inactivity threshold. The client is authenticated with an M2M JWT
// minted by the M2M client (file m2m_client.go) and uses a small
// exponential-backoff retry policy for 5xx and 429 responses.
//
// The endpoint contract comes from
// docs/services/13_Parametrizacion_Service_Spec/openapi/parametrizacion-service.yaml
// (paths./dias-inactivos/empresa/{empresa_id}, get):
//
//	GET /api/v1/parametrizacion/dias-inactivos/empresa/{empresa_id}
//	BearerAuth: []
//	200 -> CompanyInactiveDaysResponse { success, data: { empresa_id,
//	         umbral_dias, dias_inactivos[] } }
package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"math/rand"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"
)

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrParametrizacionBadRequest is returned for any 4xx response
	// except 429. The job is misconfigured or the request shape is
	// wrong; retrying will not help.
	ErrParametrizacionBadRequest = errors.New("scheduler: parametrizacion request was rejected (4xx)")
	// ErrParametrizacionUnavailable is returned when the retry
	// budget is exhausted on 5xx / 429 responses. The job should
	// surface this as a transient failure and the cron engine will
	// mark the execution FAILED.
	ErrParametrizacionUnavailable = errors.New("scheduler: parametrizacion service unavailable")
)

// ---------------------------------------------------------------------------
// Public contract.
// ---------------------------------------------------------------------------

// ParametrizacionClient reads company-scoped configuration from the
// Parametrizacion Service. Concrete implementations are concurrency-safe.
type ParametrizacionClient interface {
	// GetInactivityThreshold returns the per-company threshold of
	// consecutive inactive days after which the auto-deactivation
	// job should fire. It calls
	//   GET <baseURL>/api/v1/parametrizacion/dias-inactivos/empresa/{empresa_id}
	// with an M2M JWT carrying the "scheduler:read" scope.
	GetInactivityThreshold(ctx context.Context, empresaID int64) (int, error)
}

// ---------------------------------------------------------------------------
// HTTP implementation.
// ---------------------------------------------------------------------------

// httpParametrizacionClient is the production ParametrizacionClient.
// It holds the base URL, the M2M client used to mint the bearer
// token, and an optional structured logger. The actual HTTP
// transport is the M2M client's auto-renewing HTTPClient, so the
// bearer header is added in a single, shared place.
type httpParametrizacionClient struct {
	baseURL  string
	m2m      M2MClient
	logger   *Logger
	inner    *HTTPClient
	timeout  time.Duration
	maxTries int
	baseWait time.Duration
	factor   float64
	jitter   float64
	rng      *rand.Rand
	rngMut   chan struct{} // 1-buffered channel used as a mutex
}

// NewParametrizacionClient returns a ParametrizacionClient wired to
// the supplied dependencies. baseURL is the Parametrizacion Service
// root (e.g. "http://parametrizacion-service:8080"). m2m is the
// auto-renewing M2M client used to mint the per-request bearer
// token; the client uses m2m.HTTPClient().Do so the bearer header
// is added in a single, shared place. logger may be nil; if so,
// slog.Default() is used.
func NewParametrizacionClient(baseURL string, m2m M2MClient, logger *Logger) ParametrizacionClient {
	return &httpParametrizacionClient{
		baseURL:  strings.TrimRight(baseURL, "/"),
		m2m:      m2m,
		logger:   logger,
		inner:    m2m.HTTPClient(),
		timeout:  10 * time.Second,
		maxTries: 3, // 1 initial + 2 retries
		baseWait: 300 * time.Millisecond,
		factor:   2.0,
		jitter:   0.2,
		rng:      rand.New(rand.NewSource(time.Now().UnixNano())),
		rngMut:   make(chan struct{}, 1),
	}
}

// GetInactivityThreshold fetches the umbral_dias field for empresaID
// and returns it as a plain int. The M2M client is consulted for the
// bearer token; its cache is honoured so back-to-back calls within
// the same TTL window reuse the same token.
func (c *httpParametrizacionClient) GetInactivityThreshold(ctx context.Context, empresaID int64) (int, error) {
	if empresaID <= 0 {
		return 0, fmt.Errorf("parametrizacion_client.GetInactivityThreshold: empresa_id must be > 0 (got %d)", empresaID)
	}

	endpoint := fmt.Sprintf(
		"%s/api/v1/parametrizacion/dias-inactivos/empresa/%d",
		c.baseURL, empresaID,
	)

	var (
		umbral   int
		lastErr  error
		startAll = time.Now()
	)
	for attempt := 1; attempt <= c.maxTries; attempt++ {
		req, err := http.NewRequestWithContext(ctx, http.MethodGet, endpoint, nil)
		if err != nil {
			return 0, fmt.Errorf("parametrizacion_client.GetInactivityThreshold: new request: %w", err)
		}
		req.Header.Set("Accept", "application/json")
		// request_id propagates across retries; if one is already
		// attached via context we keep it, otherwise we mint a
		// transient one for log correlation.
		if rid := requestIDFromContext(ctx); rid != "" {
			req.Header.Set("X-Request-Id", rid)
		} else {
			req.Header.Set("X-Request-Id", newRequestID())
		}

		resp, err := c.inner.Do(req)
		if err != nil {
			lastErr = err
			c.logAttempt(ctx, "warn", empresaID, attempt, 0, 0, err)
			if attempt < c.maxTries {
				if waitErr := c.sleepBackoff(ctx, attempt); waitErr != nil {
					return 0, fmt.Errorf("parametrizacion_client.GetInactivityThreshold: context cancelled during backoff: %w", waitErr)
				}
				continue
			}
			return 0, fmt.Errorf("%w: %v", ErrParametrizacionUnavailable, err)
		}

		switch {
		case resp.StatusCode >= 200 && resp.StatusCode < 300:
			umbral, err = parseUmbralFromBody(resp.Body)
			closeErr := resp.Body.Close()
			if err != nil {
				return 0, fmt.Errorf("parametrizacion_client.GetInactivityThreshold: parse response: %w", err)
			}
			if closeErr != nil {
				return 0, fmt.Errorf("parametrizacion_client.GetInactivityThreshold: close body: %w", closeErr)
			}
			c.logAttempt(ctx, "info", empresaID, attempt, resp.StatusCode, umbral, nil)
			return umbral, nil

		case resp.StatusCode == http.StatusTooManyRequests || (resp.StatusCode >= 500 && resp.StatusCode <= 599):
			// Drain & close so the connection can be reused.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			lastErr = fmt.Errorf("parametrizacion status=%d", resp.StatusCode)
			c.logAttempt(ctx, "warn", empresaID, attempt, resp.StatusCode, 0, lastErr)
			if attempt < c.maxTries {
				if waitErr := c.sleepBackoff(ctx, attempt); waitErr != nil {
					return 0, fmt.Errorf("parametrizacion_client.GetInactivityThreshold: context cancelled during backoff: %w", waitErr)
				}
				continue
			}
			return 0, fmt.Errorf("%w: %v (status=%d, attempts=%d)", ErrParametrizacionUnavailable, lastErr, resp.StatusCode, c.maxTries)

		default:
			// 4xx other than 429: fail fast, no retry.
			_, _ = io.Copy(io.Discard, resp.Body)
			_ = resp.Body.Close()
			c.logAttempt(ctx, "warn", empresaID, attempt, resp.StatusCode, 0, nil)
			return 0, fmt.Errorf("%w: status=%d", ErrParametrizacionBadRequest, resp.StatusCode)
		}
	}
	_ = startAll // reserved for future duration_ms log; currently per-attempt.
	return 0, fmt.Errorf("%w: %v", ErrParametrizacionUnavailable, lastErr)
}

// sleepBackoff waits base * factor^(attempt-1) ± jitter*100%, or until
// the context is cancelled. attempt is 1-based; on attempt=1 the wait
// is approximately base*(1 ± jitter).
func (c *httpParametrizacionClient) sleepBackoff(ctx context.Context, attempt int) error {
	nominal := float64(c.baseWait)
	for i := 1; i < attempt; i++ {
		nominal *= c.factor
	}
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

// logAttempt emits a single structured log line per request attempt.
// The level is "info" on a 2xx, "warn" otherwise. The JWT and the
// response body are never logged.
func (c *httpParametrizacionClient) logAttempt(ctx context.Context, level string, empresaID int64, attempt, status int, umbral int, err error) {
	log := c.loggerOrSlog()
	attrs := []slog.Attr{
		slog.String("empresa_id", strconv.FormatInt(empresaID, 10)),
		slog.Int("attempt", attempt),
		slog.Int("max_attempts", c.maxTries),
	}
	if status > 0 {
		attrs = append(attrs, slog.Int("status", status))
	}
	if level == "info" {
		attrs = append(attrs,
			slog.Int("umbral_dias", umbral),
			slog.String("msg_field", "fetched inactivity threshold"),
		)
	} else if err != nil {
		attrs = append(attrs, slog.String("error", err.Error()))
	}
	switch level {
	case "info":
		log.LogAttrs(ctx, slog.LevelInfo, "fetched inactivity threshold", attrs...)
	default:
		log.LogAttrs(ctx, slog.LevelWarn, "parametrizacion request failed", attrs...)
	}
}

func (c *httpParametrizacionClient) loggerOrSlog() *slog.Logger {
	if c.logger != nil {
		return c.logger.Inner()
	}
	return slog.Default()
}

// ---------------------------------------------------------------------------
// Response envelope parser.
// ---------------------------------------------------------------------------

// companyInactiveDaysResponse mirrors the OpenAPI schema. Only the
// umbral_dias field is consumed; the rest is ignored on purpose.
type companyInactiveDaysResponse struct {
	Success bool                              `json:"success"`
	Data    *companyInactiveDaysResponse_Data `json:"data"`
}

type companyInactiveDaysResponse_Data struct {
	EmpresaID     int64            `json:"empresa_id"`
	UmbralDias    int              `json:"umbral_dias"`
	DiasInactivos []map[string]any `json:"dias_inactivos"`
}

// parseUmbralFromBody decodes the envelope and returns umbral_dias.
// It rejects empty bodies, malformed JSON, success=false, missing
// data, and umbral_dias <= 0.
func parseUmbralFromBody(body io.Reader) (int, error) {
	raw, err := io.ReadAll(body)
	if err != nil {
		return 0, fmt.Errorf("read body: %w", err)
	}
	if len(raw) == 0 {
		return 0, errors.New("empty response body")
	}
	var env companyInactiveDaysResponse
	if err := json.Unmarshal(raw, &env); err != nil {
		return 0, fmt.Errorf("unmarshal envelope: %w", err)
	}
	if !env.Success {
		return 0, errors.New("envelope success=false")
	}
	if env.Data == nil {
		return 0, errors.New("envelope missing data")
	}
	if env.Data.UmbralDias <= 0 {
		return 0, fmt.Errorf("umbral_dias must be > 0 (got %d)", env.Data.UmbralDias)
	}
	return env.Data.UmbralDias, nil
}

// ---------------------------------------------------------------------------
// Internal helpers.
// ---------------------------------------------------------------------------

// requestIDContextKey is the context key used to carry a request id
// across the call boundary. It is unexported because the scheduler
// package is the only writer.
type requestIDContextKey struct{}

// withRequestID returns a derived context carrying a request id.
func withRequestID(ctx context.Context, id string) context.Context {
	if id == "" {
		return ctx
	}
	return context.WithValue(ctx, requestIDContextKey{}, id)
}

// requestIDFromContext returns the request id stored on the context,
// or "" if absent.
func requestIDFromContext(ctx context.Context) string {
	if ctx == nil {
		return ""
	}
	if v, ok := ctx.Value(requestIDContextKey{}).(string); ok {
		return v
	}
	return ""
}

// newRequestID returns a fresh request id. We use the trace id from
// the active OTel span when available, otherwise a short random
// identifier. The function is intentionally small to keep the
// dependency surface narrow.
func newRequestID() string {
	if id := TraceIDFromContext(context.Background()); id != "" {
		return id
	}
	return fmt.Sprintf("prm-%d", time.Now().UnixNano())
}

// IsTransientError reports whether err is a transient Parametrizacion
// failure. Exposed for the cron runner / metrics layer.
func IsTransientError(err error) bool {
	return errors.Is(err, ErrParametrizacionUnavailable)
}

// IsClientError reports whether err is a permanent 4xx failure.
func IsClientError(err error) bool {
	return errors.Is(err, ErrParametrizacionBadRequest)
}

// joinPath is a tiny URL helper that respects the baseURL and the
// caller-provided path. It deliberately avoids url.URL.Join so the
// function is allocation-free and easy to read in stack traces.
func joinPath(baseURL, path string) (string, error) {
	u, err := url.Parse(baseURL)
	if err != nil {
		return "", err
	}
	u.Path = strings.TrimRight(u.Path, "/") + "/" + strings.TrimLeft(path, "/")
	return u.String(), nil
}
