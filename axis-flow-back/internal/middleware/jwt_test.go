// Package middleware — JWT claims enrichment tests for PR-5.
//
// The formularios POST /evento_iniciado endpoint requires the
// empleado_id claim to be in the access token (the formularios
// handler reads it from the context and refuses body fallbacks
// since PR-4 AMEND FIX 5). PR-5 (cross-cutting blocker 5.0)
// wires the claim into the production JWT schema and the
// JWTAuth middleware must extract it into the request context
// under ContextKeyEmpleadoID.
//
// Backward-compat invariant: tokens WITHOUT the empleado_id
// claim (issued by an older issuer, or by tests that omit the
// claim) must still parse cleanly. The middleware writes
// int64(0) into the context so the formularios extractEmpleadoID
// helper can distinguish "missing" from "zero" — the helper
// rejects BOTH, but the rest of the system (non-formularios
// endpoints) treats 0 the same as missing.
package middleware

import (
	"context"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"axis-flow-back/internal/domain"
	"axis-flow-back/internal/service"

	"github.com/golang-jwt/jwt/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

const testJWTSecret = "test-secret-key-32-bytes-longXXX"

// buildAuthSvc constructs a real *service.AuthService with minimal
// stub repos. The 6-arg NewAuthService signature (with EmpleadoLookup)
// is the one PR-5 introduces; the lookup parameter is the seam the
// tests exercise.
func buildAuthSvc(t *testing.T, lookup service.EmpleadoLookup) *service.AuthService {
	t.Helper()
	users := &stubUserRepo{}
	sess := &stubSessionRepo{}
	return service.NewAuthService(users, sess, testJWTSecret, 15*time.Minute, 7*24*time.Hour, lookup)
}

// mintToken signs a token with the test secret + the supplied claims.
// Bypasses AuthService.issueTokenPair so the tests can craft tokens
// WITHOUT the empleado_id claim (the backward-compat path).
func mintToken(t *testing.T, claims service.Claims) string {
	t.Helper()
	tok, err := jwt.NewWithClaims(jwt.SigningMethodHS256, &claims).SignedString([]byte(testJWTSecret))
	require.NoError(t, err)
	return tok
}

// ---------------------------------------------------------------------------
// Tests.
// ---------------------------------------------------------------------------

// TestJWTAuth_PopulatesEmpleadoIDFromClaim asserts that a token
// carrying the empleado_id claim has the value written into the
// request context under ContextKeyEmpleadoID.
func TestJWTAuth_PopulatesEmpleadoIDFromClaim(t *testing.T) {
	authSvc := buildAuthSvc(t, &stubEmpleadoLookup{empleadoID: 42})
	mw := JWTAuth(authSvc)

	tok := mintToken(t, service.Claims{
		UserID:     uuid.New().String(),
		TenantID:   uuid.New().String(),
		Email:      "emp@example.com",
		Role:       "EMPLEADO",
		EmpleadoID: 42,
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	var capturedID int64
	var capturedOK bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := r.Context().Value(ContextKeyEmpleadoID).(int64)
		capturedID, capturedOK = v, ok
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, capturedOK, "ContextKeyEmpleadoID must be present in the context")
	assert.Equal(t, int64(42), capturedID, "context value must equal the claim")
}

// TestJWTAuth_MissingEmpleadoClaim_WritesZero asserts backward
// compatibility: tokens WITHOUT the empleado_id claim still parse
// (the JWT signature is valid), and the middleware writes int64(0)
// into the context. The formularios extractEmpleadoID helper rejects
// 0; the rest of the system treats 0 the same as missing.
func TestJWTAuth_MissingEmpleadoClaim_WritesZero(t *testing.T) {
	authSvc := buildAuthSvc(t, &stubEmpleadoLookup{empleadoID: 0})
	mw := JWTAuth(authSvc)

	// Mint a token with the same secret but without the empleado_id
	// claim. The Claims.EmpleadoID field is its zero value (int64(0))
	// and the JSON output has no "empleado_id" key (omitempty).
	tok := mintToken(t, service.Claims{
		UserID:   uuid.New().String(),
		TenantID: uuid.New().String(),
		Email:    "no-empleado@example.com",
		Role:     "ADMIN",
		// EmpleadoID intentionally omitted → 0
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	var capturedID int64
	var capturedOK bool
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		v, ok := r.Context().Value(ContextKeyEmpleadoID).(int64)
		capturedID, capturedOK = v, ok
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	require.True(t, capturedOK, "ContextKeyEmpleadoID key must be present even when the value is 0")
	assert.Equal(t, int64(0), capturedID, "missing claim must be written as 0 (backward compat)")
}

// TestJWTAuth_PreservesExistingContextKeys asserts that the new
// ContextKeyEmpleadoID line does not regress any of the existing
// context keys (UserID, TenantID, Email, Role).
func TestJWTAuth_PreservesExistingContextKeys(t *testing.T) {
	authSvc := buildAuthSvc(t, &stubEmpleadoLookup{empleadoID: 7})
	mw := JWTAuth(authSvc)

	userID := uuid.New()
	tenantID := uuid.New()
	tok := mintToken(t, service.Claims{
		UserID:     userID.String(),
		TenantID:   tenantID.String(),
		Email:      "preserve@example.com",
		Role:       "Manager",
		EmpleadoID: 7,
	})

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer "+tok)
	rec := httptest.NewRecorder()

	var got service.Claims
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		uid, _ := r.Context().Value(ContextKeyUserID).(string)
		tid, _ := r.Context().Value(ContextKeyTenantID).(string)
		em, _ := r.Context().Value(ContextKeyEmail).(string)
		ro, _ := r.Context().Value(ContextKeyRole).(string)
		empID, _ := r.Context().Value(ContextKeyEmpleadoID).(int64)
		got = service.Claims{UserID: uid, TenantID: tid, Email: em, Role: ro, EmpleadoID: empID}
		w.WriteHeader(http.StatusOK)
	}))
	handler.ServeHTTP(rec, req)

	require.Equal(t, http.StatusOK, rec.Code)
	assert.Equal(t, userID.String(), got.UserID)
	assert.Equal(t, tenantID.String(), got.TenantID)
	assert.Equal(t, "preserve@example.com", got.Email)
	assert.Equal(t, "Manager", got.Role)
	assert.Equal(t, int64(7), got.EmpleadoID)
}

// TestJWTAuth_InvalidToken_Returns401 asserts that the new context
// line does not affect the failure path: an invalid signature is
// still 401 with no body leakage.
func TestJWTAuth_InvalidToken_Returns401(t *testing.T) {
	authSvc := buildAuthSvc(t, &stubEmpleadoLookup{empleadoID: 0})
	mw := JWTAuth(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	req.Header.Set("Authorization", "Bearer not-a-jwt")
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler must NOT be called for an invalid token")
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// TestJWTAuth_MissingAuthHeader_Returns401 covers the no-header path
// to ensure the EmpleadoID line does not accidentally swallow the
// 401 when the header is missing.
func TestJWTAuth_MissingAuthHeader_Returns401(t *testing.T) {
	authSvc := buildAuthSvc(t, &stubEmpleadoLookup{empleadoID: 0})
	mw := JWTAuth(authSvc)

	req := httptest.NewRequest(http.MethodGet, "/x", nil)
	rec := httptest.NewRecorder()
	mw(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		t.Fatal("downstream handler must NOT be called when the auth header is missing")
	})).ServeHTTP(rec, req)

	assert.Equal(t, http.StatusUnauthorized, rec.Code)
}

// ---------------------------------------------------------------------------
// Stub repositories.
// ---------------------------------------------------------------------------

// stubEmpleadoLookup is the auth service's EmpleadoLookup seam. Returns
// the configured empleadoID; if findErr is set, returns the error.
type stubEmpleadoLookup struct {
	empleadoID int64
	findErr    error
	calls      int
}

func (s *stubEmpleadoLookup) GetEmpleadoIDByUserID(_ context.Context, _, _ uuid.UUID) (int64, error) {
	s.calls++
	return s.empleadoID, s.findErr
}

// stubUserRepo satisfies service.UserRepository with no-op
// implementations. ParseAccessToken does not call the repo, so the
// stubs are only here to satisfy the type system.
type stubUserRepo struct{}

func (s *stubUserRepo) FindByEmail(_ context.Context, _ string) (*domain.User, error) {
	return nil, assert.AnError
}
func (s *stubUserRepo) FindByID(_ context.Context, _ uuid.UUID) (*domain.User, error) {
	return nil, assert.AnError
}
func (s *stubUserRepo) FindPrimaryRoleCode(_ context.Context, _ uuid.UUID) (string, error) {
	return "EMPLEADO", nil
}
func (s *stubUserRepo) ListPermissionCodes(_ context.Context, _ uuid.UUID) ([]string, error) {
	return nil, nil
}
func (s *stubUserRepo) UpdateStatus(_ context.Context, _ uuid.UUID, _ domain.UserStatus) error {
	return nil
}
func (s *stubUserRepo) UpdateLastLogin(_ context.Context, _ uuid.UUID, _ time.Time) error {
	return nil
}
func (s *stubUserRepo) UpdatePasswordHash(_ context.Context, _ uuid.UUID, _ string) error {
	return nil
}
func (s *stubUserRepo) IncrementFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}
func (s *stubUserRepo) ResetFailedAttempts(_ context.Context, _ uuid.UUID) error {
	return nil
}

// stubSessionRepo satisfies service.SessionRepository with no-op
// implementations.
type stubSessionRepo struct{}

func (s *stubSessionRepo) Create(_ context.Context, _ *domain.Session) error { return nil }
func (s *stubSessionRepo) FindByRefreshTokenHash(_ context.Context, _ string) (*domain.Session, error) {
	return nil, assert.AnError
}
func (s *stubSessionRepo) Revoke(_ context.Context, _ uuid.UUID) error { return nil }
