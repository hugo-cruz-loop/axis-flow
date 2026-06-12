// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the M2M (machine-to-machine) JWT client and the
// auto-renewing HTTPClient wrapper described in the spec section
// "Seguridad > Zero Trust Job Routing". Every outbound HTTP call the
// scheduler makes to internal microservices carries a short-lived
// scoped JWT signed with a shared service key. Receiving services
// validate iss/aud/scope and enforce access boundaries.
//
// The HTTPClient is the preferred call path: callers that need only
// the token (e.g. the parametrizacion client, Phase 2B task 2.7) may
// go through HTTPClient directly. The standalone MintToken method is
// exposed for callers that have their own transport and only need a
// signed token.
package service

import (
	"context"
	"errors"
	"fmt"
	"net/http"
	"sync"
	"time"

	"axis-flow-back/internal/scheduler"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrM2MSigningKeyMissing is returned by NewM2MClient when the
	// supplied config lacks a signing key. The scheduler.config layer
	// already enforces this in Validate(); the runtime sentinel is a
	// belt-and-braces guard so direct callers (tests) get a typed
	// error.
	ErrM2MSigningKeyMissing = errors.New("scheduler: M2M signing key missing or too short")
	// ErrM2MTokenMint is returned by MintToken when the underlying
	// jwt library fails. The original error is wrapped with %w so
	// callers can errors.Is / errors.As on the underlying cause.
	ErrM2MTokenMint = errors.New("scheduler: failed to mint M2M token")
)

// M2MConfig is the subset of scheduler.Config that the M2M client
// needs. It is intentionally narrow so callers can construct one
// without leaking other scheduler fields.
type M2MConfig struct {
	SigningKey []byte
	KeyID      string
	Audience   string
	Issuer     string
}

// ---------------------------------------------------------------------------
// Public contract.
// ---------------------------------------------------------------------------

// M2MClient mints scoped JWTs and exposes an auto-renewing HTTP
// transport. Concrete implementations are concurrency-safe.
type M2MClient interface {
	// MintToken returns a fresh JWT carrying the supplied scopes and
	// the requested TTL. ttl is clamped to the [1m, 1h] range to
	// avoid pathological values (e.g. zero or 24h).
	MintToken(ctx context.Context, scopes []string, ttl time.Duration) (string, error)
	// AcquireToken returns an up-to-date token, using the cache when
	// the cached one is still valid and minting a fresh token
	// otherwise. It is the method HTTPClient.Do calls on every
	// request to obtain the bearer value. Exposed on the interface
	// so test doubles (and future M2M backends) can plug in.
	AcquireToken(ctx context.Context) (string, error)
	// HTTPClient returns the auto-renewing wrapper. The same wrapper
	// is returned on every call; it is safe for concurrent use by
	// many goroutines.
	HTTPClient() *HTTPClient
}

// ---------------------------------------------------------------------------
// JWT minting + auto-renewal state.
// ---------------------------------------------------------------------------

// jwtM2MClient is the production M2MClient. It holds the signing key
// in memory, the configured audience/issuer, and a single cached
// token + expiry used by the auto-renewing HTTPClient wrapper.
type jwtM2MClient struct {
	signingKey []byte
	keyID      string
	audience   string
	issuer     string

	// Cached token + expiry. The HTTPClient wrapper consults the
	// cache first; it mints a new one when the remaining lifetime
	// is below the renew threshold.
	mu        sync.Mutex
	cachedTok string
	cachedExp time.Time
	// renewBefore is the minimum remaining lifetime below which a
	// cached token is replaced. 30s matches the spec.
	renewBefore time.Duration
	// httpClient is the wrapper returned by HTTPClient.
	httpClient *HTTPClient
}

// NewM2MClient constructs an M2MClient from a scheduler.Config. The
// signing key is read into a []byte and copied so callers may zero
// the source. Returns ErrM2MSigningKeyMissing when the key is empty
// or shorter than 32 bytes — matching the Validate() contract in
// config.go.
//
// audience is defaulted to "axis-flow-internal" when empty; issuer is
// always "scheduler-service" and cannot be overridden: it is the
// contract the receiving services use to identify the caller.
func NewM2MClient(cfg scheduler.Config) M2MClient {
	// Defensive copy so future refactors that re-use the Config
	// cannot leak the secret via the original backing array.
	key := make([]byte, len(cfg.M2MSigningKey))
	copy(key, cfg.M2MSigningKey)

	audience := cfg.M2MAudience
	if audience == "" {
		audience = "axis-flow-internal"
	}

	c := &jwtM2MClient{
		signingKey:  key,
		keyID:       cfg.M2MKeyID,
		audience:    audience,
		issuer:      "scheduler-service",
		renewBefore: 30 * time.Second,
	}
	c.httpClient = &HTTPClient{inner: http.DefaultClient, m2m: c}
	return c
}

// MintToken builds a JWT with the standard registered claims plus a
// custom scope claim (space-separated). The token is signed with
// HS256 using the configured shared key.
func (c *jwtM2MClient) MintToken(ctx context.Context, scopes []string, ttl time.Duration) (string, error) {
	if len(c.signingKey) < 32 {
		return "", fmt.Errorf("jwtM2MClient.MintToken: %w", ErrM2MSigningKeyMissing)
	}
	if ttl <= 0 {
		ttl = 5 * time.Minute
	}
	if ttl < time.Minute {
		ttl = time.Minute
	}
	if ttl > time.Hour {
		ttl = time.Hour
	}

	now := time.Now().UTC()
	claims := jwt.MapClaims{
		"iss":   c.issuer,
		"aud":   c.audience,
		"sub":   c.issuer,
		"iat":   now.Unix(),
		"exp":   now.Add(ttl).Unix(),
		"jti":   uuid.NewString(),
		"scope": scopeClaim(scopes),
	}
	tok := jwt.NewWithClaims(jwt.SigningMethodHS256, claims)
	if c.keyID != "" {
		tok.Header["kid"] = c.keyID
	}
	signed, err := tok.SignedString(c.signingKey)
	if err != nil {
		return "", fmt.Errorf("jwtM2MClient.MintToken: %w: %v", ErrM2MTokenMint, err)
	}
	return signed, nil
}

// HTTPClient returns the wrapper. The same wrapper is returned on
// every call so its internal state is shared.
func (c *jwtM2MClient) HTTPClient() *HTTPClient { return c.httpClient }

// AcquireToken returns the cached token if it has at least
// renewBefore seconds of life left, otherwise mints a new one and
// updates the cache. Exposed on the M2MClient interface so
// HTTPClient.Do and other transports can obtain a fresh token
// without reaching into the unexported cache.
func (c *jwtM2MClient) AcquireToken(ctx context.Context) (string, error) {
	return c.currentToken(ctx)
}

// ---------------------------------------------------------------------------
// HTTPClient — auto-renewing Authorization: Bearer wrapper.
// ---------------------------------------------------------------------------

// HTTPClient wraps an inner *http.Client and injects
// "Authorization: Bearer <token>" on every request. The token comes
// from the M2MClient and is auto-renewed when its remaining lifetime
// drops below the renewBefore threshold.
type HTTPClient struct {
	inner *http.Client
	m2m   M2MClient
}

// Do runs the request with an up-to-date bearer token. The token is
// obtained from m2m.AcquireToken — the same method the M2M client
// itself uses internally for its own HTTPClient wrapper, so the
// caching and auto-renewal logic is in one place.
func (h *HTTPClient) Do(req *http.Request) (*http.Response, error) {
	if h == nil || h.m2m == nil {
		return nil, fmt.Errorf("HTTPClient.Do: client not initialised")
	}
	if req == nil {
		return nil, fmt.Errorf("HTTPClient.Do: request is nil")
	}
	tok, err := h.m2m.AcquireToken(req.Context())
	if err != nil {
		return nil, fmt.Errorf("HTTPClient.Do: mint token: %w", err)
	}
	// Clone the header so the caller's request is not mutated.
	req.Header = req.Header.Clone()
	req.Header.Set("Authorization", "Bearer "+tok)
	return h.inner.Do(req)
}

// currentToken returns the cached token when it has at least
// renewBefore seconds of life left; otherwise it mints a new one and
// updates the cache.
func (c *jwtM2MClient) currentToken(ctx context.Context) (string, error) {
	c.mu.Lock()
	defer c.mu.Unlock()
	if c.cachedTok != "" && time.Until(c.cachedExp) > c.renewBefore {
		return c.cachedTok, nil
	}
	const ttl = 5 * time.Minute
	tok, err := c.MintToken(ctx, nil, ttl)
	if err != nil {
		return "", err
	}
	c.cachedTok = tok
	c.cachedExp = time.Now().Add(ttl)
	return tok, nil
}

// scopeClaim joins non-empty scopes with a single space. The OAuth 2
// / RFC 8693 convention is space-separated; an empty input yields the
// empty string so the claim remains present (some validators reject a
// missing scope).
func scopeClaim(scopes []string) string {
	out := ""
	for i, s := range scopes {
		s = trimSpace(s)
		if s == "" {
			continue
		}
		if i > 0 && out != "" {
			out += " "
		}
		out += s
	}
	return out
}

// trimSpace is a local helper to avoid importing strings just for
// one call. Kept as a tiny pure function for testability.
func trimSpace(s string) string {
	start, end := 0, len(s)
	for start < end && isSpace(s[start]) {
		start++
	}
	for end > start && isSpace(s[end-1]) {
		end--
	}
	return s[start:end]
}

func isSpace(c byte) bool {
	return c == ' ' || c == '\t' || c == '\n' || c == '\r' || c == '\v' || c == '\f'
}
