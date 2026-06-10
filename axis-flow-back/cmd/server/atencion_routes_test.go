package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestAtencionRouter wires a minimal chi.Router with the atencion routes
// mounted. The real JWT middleware is injected so that unauthenticated requests
// produce 401 as expected.
func buildTestAtencionRouter(t *testing.T) chi.Router {
	t.Helper()
	r := chi.NewRouter()

	// Use the real JWT-returning middleware to exercise auth enforcement.
	// We inject a sentinel middleware that always returns 401 so we don't
	// need a real token or auth service in these smoke tests.
	alwaysUnauthorized := func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
		})
	}
	noopRoles := func(_ ...string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler { return next }
	}

	mod := newAtencionModuleForTest()
	registerAtencionRoutes(r, mod, alwaysUnauthorized, noopRoles)
	return r
}

func TestAtencionRoutes_PostQuejaWithoutJWT_Returns401(t *testing.T) {
	r := buildTestAtencionRouter(t)

	req := httptest.NewRequest(http.MethodPost, "/api/v1/atencion/queja", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.NotEqual(t, http.StatusNotFound, w.Code, "route POST /api/v1/atencion/queja is not registered")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAtencionRoutes_GetQuejaWithoutJWT_Returns401(t *testing.T) {
	r := buildTestAtencionRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/atencion/queja/00000000-0000-0000-0000-000000000001", nil)
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	require.NotEqual(t, http.StatusNotFound, w.Code, "route GET /api/v1/atencion/queja/{id} is not registered")
	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestAtencionRoutes_AllRoutesRegistered(t *testing.T) {
	r := buildTestAtencionRouter(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/atencion/queja"},
		{http.MethodGet, "/api/v1/atencion/queja/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/api/v1/atencion/queja/00000000-0000-0000-0000-000000000001/mensajes"},
		{http.MethodPost, "/api/v1/atencion/queja/00000000-0000-0000-0000-000000000001/mensaje"},
		{http.MethodGet, "/api/v1/atencion/empresa/00000000-0000-0000-0000-000000000001/quejas"},
		{http.MethodPost, "/api/v1/atencion/ticket"},
		{http.MethodGet, "/api/v1/atencion/ticket/00000000-0000-0000-0000-000000000001/mensajes"},
		{http.MethodPost, "/api/v1/atencion/ticket/00000000-0000-0000-0000-000000000001/mensaje"},
		{http.MethodPatch, "/api/v1/atencion/ticket/00000000-0000-0000-0000-000000000001/status"},
		{http.MethodGet, "/api/v1/atencion/cliente/00000000-0000-0000-0000-000000000001/tickets"},
		{http.MethodGet, "/api/v1/atencion/empresa/00000000-0000-0000-0000-000000000001/tickets/stats"},
		{http.MethodPost, "/api/v1/atencion/incidencia"},
		{http.MethodGet, "/api/v1/atencion/empresa/00000000-0000-0000-0000-000000000001/incidencias"},
	}

	for _, tc := range cases {
		req := httptest.NewRequest(tc.method, tc.path, nil)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.NotEqualf(t, http.StatusNotFound, w.Code, "%s %s is not registered", tc.method, tc.path)
	}
}
