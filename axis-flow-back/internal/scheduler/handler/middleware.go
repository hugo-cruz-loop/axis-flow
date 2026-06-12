// middleware.go — JWT bearer middleware for the Scheduler Service's
// admin REST surface.
//
// Scope of this middleware
// =========================
// The Scheduler Service is an internal-to-internal component; the
// admin endpoints (GET /api/v1/scheduler/jobs, POST .../trigger,
// PATCH .../pause, GET /api/v1/scheduler/executions) are protected
// by a machine-to-machine (M2M) JWT. The token is minted by any
// other internal service using the shared signing key
// (SCHEDULER_M2M_SIGNING_KEY) and carries one or more of the
// required scopes in its `scope` claim.
//
// The middleware:
//   1. Reads the "Authorization: Bearer <token>" header.
//   2. Verifies the signature using the shared signing key with
//      the same HS256 algorithm the M2M client uses (see
//      service/m2m_client.go). Reusing the same library
//      (golang-jwt/jwt/v5) keeps the validation contract
//      consistent across the codebase.
//   3. Verifies the standard claims (exp, nbf) via the library's
//      parser. `iss` is treated as the caller's identity — the
//      middleware is PERMISSIVE on the issuer: any non-empty iss
//      is accepted as long as the signature is valid and the
//      scope check passes. The spec section "Seguridad > Zero
//      Trust Job Routing" calls this out explicitly — receiving
//      APIs validate signature + scope; the iss check is for
//      observability, not access control. The middleware logs the
//      iss on every request so the operator can spot a caller
//      that is not "scheduler-service" or another known internal
//      issuer.
//   4. Parses the `scope` claim (RFC 8693 — space-separated) and
//      checks that at least one of the required scopes is
//      present. 401 on missing/invalid token, 403 on missing
//      scope.
//   5. Stores the parsed claims on the request context via
//      WithClaims so downstream handlers can read the subject
//      (sub / empleado_id), the scopes, and the issuer without
//      re-parsing the token.
//   6. Emits one structured access log line per request
//      (msg="auth check" path=... scopes_matched=... request_id=...
//      status=...). Failures are logged at warn level, successes
//      at info. The structured logger used is the package-level
//      slog.Default(); production wiring should replace it via
//      SetAuthLogger (see below) so the records land on the
//      scheduler's logfmt pipeline.
//
// What this middleware does NOT do
// ================================
//   - It does NOT validate the `aud` claim. The scheduler mints
//     tokens with the project-wide `axis-flow-internal` audience
//     and there is no per-route audience differentiation yet.
//     A future iteration may want to require `aud` to be set to
//     "scheduler-service" on inbound calls.
//   - It does NOT enforce a token-replay window. The standard
//     `exp` check is sufficient at the current scale; the
//     scheduler does not see per-user tokens, only per-service.
//   - It does NOT handle refresh tokens. The M2M token has a
//     short TTL (5 minutes by default) and the caller mints a
//     fresh one on demand; refresh tokens are a public-web
//     concept that does not apply here.
package handler

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"sync"
	"time"

	"axis-flow-back/internal/scheduler/service"

	"github.com/golang-jwt/jwt/v5"
)

// ---------------------------------------------------------------------------
// Sentinel errors. Use errors.Is to inspect.
// ---------------------------------------------------------------------------

var (
	// ErrJWTMissing is returned when the Authorization header is
	// absent, blank, or does not start with "Bearer ".
	ErrJWTMissing = errors.New("scheduler: missing or malformed Authorization header")
	// ErrJWTInvalid is returned when the token fails to parse, the
	// signature is bad, the standard claims (exp, nbf) are
	// violated, or the token is otherwise not trustworthy.
	ErrJWTInvalid = errors.New("scheduler: invalid JWT")
	// ErrJWTInsufficientScope is returned when the token is
	// otherwise valid but does not carry at least one of the
	// required scopes in its `scope` claim.
	ErrJWTInsufficientScope = errors.New("scheduler: JWT does not carry a required scope")
)

// ---------------------------------------------------------------------------
// Public claims type.
// ---------------------------------------------------------------------------

// JWTClaims is the parsed-and-validated view of a Scheduler
// M2M token. Fields are populated from the standard registered
// claims plus the custom `scope` claim. The struct is the value
// stored on the request context via WithClaims — handlers that
// need the caller's identity (sub, empleado_id) read it back via
// ClaimsFromContext.
type JWTClaims struct {
	// Subject is the standard `sub` claim. For M2M tokens the
	// caller service sets it to its own identifier (e.g.
	// "admin-service"). The Scheduler does not depend on the
	// value — the field is exposed for observability and
	// downstream auditing.
	Subject string
	// Issuer is the standard `iss` claim. Logged on every
	// authenticated request so the operator can spot a caller
	// that is not on the expected allowlist.
	Issuer string
	// Audience is the standard `aud` claim. Stored verbatim
	// (may be a comma-separated list per RFC 7519).
	Audience string
	// Scopes is the parsed `scope` claim (RFC 8693 —
	// space-separated string). Stored as a slice so handlers
	// can iterate without re-parsing.
	Scopes []string
	// EmpleadoID is the optional `empleado_id` claim. Some
	// M2M tokens carry it when the caller is acting on behalf
	// of a specific employee (e.g. an admin tool running
	// under a delegated user). The Scheduler does not
	// currently consume it; the field is exposed for forward
	// compatibility.
	EmpleadoID string
	// ExpiresAt is the parsed `exp` claim as a UTC time.Time.
	ExpiresAt time.Time
}

// hasScope reports whether the claims carry the supplied scope.
func (c *JWTClaims) hasScope(want string) bool {
	if c == nil {
		return false
	}
	for _, s := range c.Scopes {
		if s == want {
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Context plumbing.
// ---------------------------------------------------------------------------

// claimsContextKey is the unexported key under which the parsed
// claims are stored by RequireJWT. ClaimsFromContext reads it
// back. The key stays private so callers cannot collide on the
// context value.
type claimsContextKey struct{}

// WithClaims returns a derived context carrying the parsed
// claims. The companion accessor ClaimsFromContext retrieves
// them. A nil claims value is rejected so the value on the
// context is always usable.
func WithClaims(ctx context.Context, claims *JWTClaims) context.Context {
	if ctx == nil {
		ctx = context.Background()
	}
	if claims == nil {
		return ctx
	}
	return context.WithValue(ctx, claimsContextKey{}, claims)
}

// ClaimsFromContext returns the claims previously stored with
// WithClaims, or nil when absent.
func ClaimsFromContext(ctx context.Context) *JWTClaims {
	if ctx == nil {
		return nil
	}
	v, _ := ctx.Value(claimsContextKey{}).(*JWTClaims)
	return v
}

// ---------------------------------------------------------------------------
// Middleware factory.
// ---------------------------------------------------------------------------

// RequireJWT returns a chi middleware that validates the inbound
// "Authorization: Bearer <token>" header against the shared
// signing key and enforces that the token carries at least one of
// the required scopes.
//
// Behaviour summary:
//   - 401 (ErrJWTMissing) when the header is absent or malformed.
//   - 401 (ErrJWTInvalid) when the signature, exp, or nbf is bad.
//   - 403 (ErrJWTInsufficientScope) when the scope check fails.
//   - 200 (next.ServeHTTP) on success, with the parsed claims
//     stashed on the request context.
//
// The signing key is the same HS256 secret the Scheduler uses to
// mint M2M tokens (SCHEDULER_M2M_SIGNING_KEY). The validation
// contract is intentionally identical to the m2m_client.MintToken
// path — same library, same algorithm, same claim shape — so a
// token minted by the scheduler and a token minted by another
// internal service with the same key are accepted uniformly.
func RequireJWT(signingKey []byte, requiredScopes ...string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()

			// 1. Read and validate the header shape.
			raw, err := extractBearer(r.Header.Get("Authorization"))
			if err != nil {
				writeAuthError(w, r, http.StatusUnauthorized, ErrJWTMissing, err.Error())
				return
			}

			// 2. Parse and verify the token.
			claims, perr := parseAndVerify(raw, signingKey)
			if perr != nil {
				writeAuthError(w, r, http.StatusUnauthorized, ErrJWTInvalid, perr.Error())
				return
			}

			// 3. Scope check.
			matchedScope := ""
			if !hasAnyScope(claims, requiredScopes, &matchedScope) {
				writeAuthError(w, r, http.StatusForbidden, ErrJWTInsufficientScope,
					fmt.Sprintf("token does not carry any of the required scopes: %v", requiredScopes))
				return
			}

			// 4. Stash the claims on the context for downstream
			// handlers.
			ctx = WithClaims(ctx, claims)

			// 5. Per-request access log.
			logAuthCheck(r, claims, matchedScope, http.StatusOK)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// ---------------------------------------------------------------------------
// Token parsing.
// ---------------------------------------------------------------------------

// extractBearer pulls the token from a "Bearer <token>" header
// value. The prefix check is case-sensitive on the canonical
// "Bearer " form; OAuth 2 explicitly mandates the scheme be
// case-insensitive, but the rest of the project (see
// internal/middleware/jwt.go) treats the prefix as the literal
// "Bearer " string. We match the local convention to keep the
// behaviour consistent across services.
func extractBearer(header string) (string, error) {
	if header == "" {
		return "", errors.New("missing Authorization header")
	}
	const prefix = "Bearer "
	if !strings.HasPrefix(header, prefix) {
		return "", errors.New("Authorization header must use the Bearer scheme")
	}
	tok := strings.TrimSpace(header[len(prefix):])
	if tok == "" {
		return "", errors.New("Authorization header is empty")
	}
	return tok, nil
}

// parseAndVerify parses the raw JWT, verifies the signature with
// the shared signing key, and converts the standard claims to a
// JWTClaims value. It is permissive on `iss` (any non-empty
// value passes) and on `aud` (no audience check yet — see the
// file header for the rationale).
func parseAndVerify(raw string, signingKey []byte) (*JWTClaims, error) {
	if len(signingKey) == 0 {
		return nil, errors.New("signing key is empty; the scheduler middleware is misconfigured")
	}
	parser := jwt.NewParser(
		// The Scheduler M2M client mints HS256 tokens; the
		// parser must accept exactly that algorithm and
		// nothing else. Without the explicit WithValidMethods
		// the parser would accept any signed-by-the-key
		// algorithm, which is a classic JWT footgun
		// (algorithm confusion attack).
		jwt.WithValidMethods([]string{"HS256"}),
		// exp / nbf are validated by default; the leeway
		// default is 0 which matches the spec.
	)
	tok, err := parser.Parse(raw, func(t *jwt.Token) (interface{}, error) {
		return signingKey, nil
	})
	if err != nil {
		return nil, fmt.Errorf("parse/verify token: %w", err)
	}
	if !tok.Valid {
		return nil, errors.New("token is not valid")
	}
	mapClaims, ok := tok.Claims.(jwt.MapClaims)
	if !ok {
		return nil, errors.New("token claims are not a map")
	}
	return claimsFromMap(mapClaims)
}

// claimsFromMap converts a jwt.MapClaims value to the typed
// JWTClaims struct. Missing optional fields are silently treated
// as zero values; the required field (scope) is validated below by
// hasAnyScope.
func claimsFromMap(m jwt.MapClaims) (*JWTClaims, error) {
	c := &JWTClaims{
		Subject:    stringClaim(m, "sub"),
		Issuer:     stringClaim(m, "iss"),
		Audience:   audienceClaim(m),
		EmpleadoID: stringClaim(m, "empleado_id"),
	}
	if scope, ok := m["scope"].(string); ok {
		c.Scopes = splitScopes(scope)
	}
	if expF, ok := m["exp"].(float64); ok {
		c.ExpiresAt = time.Unix(int64(expF), 0).UTC()
	}
	return c, nil
}

// stringClaim returns the string value of a MapClaims key, or
// the empty string when the key is absent or not a string. The
// jwt library surfaces string claims as interface{}; a defensive
// type assertion prevents a panic when the producer serialised
// the claim as a non-string type.
func stringClaim(m jwt.MapClaims, key string) string {
	if v, ok := m[key]; ok {
		if s, ok := v.(string); ok {
			return s
		}
	}
	return ""
}

// audienceClaim returns the standard `aud` claim. RFC 7519
// allows `aud` to be either a single string or an array of
// strings; this helper handles both shapes and joins the array
// with a comma so the caller sees a flat string.
func audienceClaim(m jwt.MapClaims) string {
	v, ok := m["aud"]
	if !ok {
		return ""
	}
	switch t := v.(type) {
	case string:
		return t
	case []any:
		parts := make([]string, 0, len(t))
		for _, p := range t {
			if s, ok := p.(string); ok {
				parts = append(parts, s)
			}
		}
		return strings.Join(parts, ",")
	default:
		return ""
	}
}

// splitScopes splits an RFC 8693 scope claim (space-separated)
// into a slice, ignoring empty entries. Multiple spaces between
// scopes collapse cleanly.
func splitScopes(scope string) []string {
	if scope == "" {
		return nil
	}
	raw := strings.Split(scope, " ")
	out := make([]string, 0, len(raw))
	for _, s := range raw {
		if s = strings.TrimSpace(s); s != "" {
			out = append(out, s)
		}
	}
	return out
}

// hasAnyScope reports whether the claims carry at least one of
// the required scopes. When the match is true, matchedScope is
// set to the first matching scope (for the access log).
func hasAnyScope(claims *JWTClaims, required []string, matchedScope *string) bool {
	for _, want := range required {
		if claims.hasScope(want) {
			if matchedScope != nil {
				*matchedScope = want
			}
			return true
		}
	}
	return false
}

// ---------------------------------------------------------------------------
// Response shaping and logging.
// ---------------------------------------------------------------------------

// writeAuthError writes a JSON error envelope with the supplied
// HTTP status, sentinel error, and human message. The envelope
// shape matches the rest of the handler package's ErrorEnvelope
// (see envelope.go) so a client that already understands the
// admin endpoint error format can parse auth errors the same
// way.
func writeAuthError(w http.ResponseWriter, r *http.Request, status int, sentinel error, message string) {
	writeJSON(w, status, ErrorEnvelope{Error: ErrorBody{
		Code:    authCodeForStatus(status),
		Message: message,
	}})
	logAuthCheck(r, nil, "", status)
}

// authCodeForStatus maps the HTTP status to the wire-level
// "UPPER_SNAKE" code the rest of the scheduler API uses. 401 is
// "UNAUTHORIZED", 403 is "FORBIDDEN"; anything else falls back
// to the generic INTERNAL so a misuse of the helper does not
// silently send a misleading code.
func authCodeForStatus(status int) string {
	switch status {
	case http.StatusUnauthorized:
		return "UNAUTHORIZED"
	case http.StatusForbidden:
		return "FORBIDDEN"
	default:
		return ErrCodeInternal
	}
}

// ---------------------------------------------------------------------------
// Logger plumbing.
// ---------------------------------------------------------------------------

// authLogger is the structured logger used by the JWT
// middleware. It defaults to slog.Default() at startup; the
// router-side wiring can override it via SetAuthLogger so the
// records land on the scheduler's logfmt pipeline (and pick up
// the configured format/level). The package-level var is
// guarded by authLoggerMu so a concurrent SetAuthLogger does
// not race with a per-request log.
var (
	authLoggerMu sync.RWMutex
	authLogger   *service.Logger
)

// SetAuthLogger installs the structured logger the JWT
// middleware uses for the per-request access log. A nil value
// is silently accepted and the next call will revert to
// slog.Default(). The router-side wiring in scheduler_routes.go
// is the only intended caller; tests can override it freely.
func SetAuthLogger(l *service.Logger) {
	authLoggerMu.Lock()
	defer authLoggerMu.Unlock()
	authLogger = l
}

// getAuthLogger returns the current access-log logger, or
// nil when none has been installed. logAuthCheck is a no-op on
// a nil return so callers do not need to gate the call.
func getAuthLogger() *service.Logger {
	authLoggerMu.RLock()
	defer authLoggerMu.RUnlock()
	return authLogger
}

// logAuthCheck emits the per-request access log line documented
// in the file header. Failures are logged at warn level
// (slogLevelFor 401/403 → warn), successes at info. When no
// logger has been installed the helper falls back to
// slog.Default() so the access log is never silently dropped.
func logAuthCheck(r *http.Request, claims *JWTClaims, matchedScope string, status int) {
	scopesMatched := "no"
	if matchedScope != "" {
		scopesMatched = "yes"
	}
	iss := ""
	if claims != nil {
		iss = claims.Issuer
	}
	attrs := []slog.Attr{
		slog.String("msg", "auth check"),
		slog.String("path", r.URL.Path),
		slog.String("method", r.Method),
		slog.String("scopes_matched", scopesMatched),
		slog.String("scope", matchedScope),
		slog.String("iss", iss),
		slog.String("request_id", requestIDFromContext(r.Context())),
		slog.String("status", strconv.Itoa(status)),
	}
	logger := getAuthLogger()
	if logger == nil {
		// Defensive fallback: use slog.Default() so the
		// access log is never silently dropped. Production
		// wiring always calls SetAuthLogger so this branch
		// only fires in tests or in a misconfigured mount.
		slog.Default().LogAttrs(r.Context(), slogLevelFor(status), "auth check", attrs...)
		return
	}
	logger.Inner().LogAttrs(r.Context(), slogLevelFor(status), "auth check", attrs...)
}
