package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/config"

	"github.com/go-chi/chi/v5"
)

// TestRegisterCursosRoutes_NoPanic verifies that newCursosModule + registerCursosRoutes
// complete without panicking when given real (but test-local) dependencies are nil-safe.
// We use nil for the pool/rdb because the test only exercises routing registration — no DB
// calls are made.
func TestRegisterCursosRoutes_NoPanic(t *testing.T) {
	cfg := &config.Config{}

	// newCursosModule must not panic with zero config (all repos/services constructed
	// from nil pool/rdb are not called during registration).
	m := newCursosModule(nil, nil, cfg)

	r := chi.NewRouter()
	// Supply no-op middleware factories that match the expected signatures.
	noopJWT := func(next http.Handler) http.Handler { return next }
	noopRequireRoles := func(_ ...string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler { return next }
	}

	// Must not panic.
	registerCursosRoutes(r, m, noopJWT, noopRequireRoles)
}

// TestGetCursosRequires401WithoutJWT verifies that GET /api/v1/cursos returns 401
// when no Authorization header is present (JWT middleware rejects the request).
func TestGetCursosRequires401WithoutJWT(t *testing.T) {
	cfg := &config.Config{}
	m := newCursosModule(nil, nil, cfg)

	r := chi.NewRouter()

	// JWT middleware that always returns 401 (simulates real JWTAuth behaviour).
	rejectAll := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, req *http.Request) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		})
	}
	noopRequireRoles := func(_ ...string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler { return next }
	}

	registerCursosRoutes(r, m, rejectAll, noopRequireRoles)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/cursos", nil)
	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, req)

	if rec.Code != http.StatusUnauthorized {
		t.Fatalf("expected 401, got %d", rec.Code)
	}
}
