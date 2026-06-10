package main

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// buildTestBolsaTrabajoRouter wires a minimal chi.Router with the bolsatrabajo
// routes mounted using no-op auth helpers — sufficient to verify route registration.
func buildTestBolsaTrabajoRouter(t *testing.T) chi.Router {
	t.Helper()
	r := chi.NewRouter()

	// No-op auth middlewares so tests are auth-agnostic.
	noopJWT := func(next http.Handler) http.Handler { return next }
	noopRoles := func(_ ...string) func(http.Handler) http.Handler {
		return func(next http.Handler) http.Handler { return next }
	}

	mod := newBolsaTrabajoModuleForTest()
	registerBolsaTrabajoRoutes(r, mod, noopJWT, noopRoles)
	return r
}

func TestBolsaTrabajoRoutes_ActiveJobsIsPublicAndNotMissing(t *testing.T) {
	r := buildTestBolsaTrabajoRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bolsa-trabajo/trabajo/activeJobs", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	// Route must be registered (not 404) and must succeed without auth (200).
	require.NotEqual(t, http.StatusNotFound, w.Code, "route /api/v1/bolsa-trabajo/trabajo/activeJobs is not registered")
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBolsaTrabajoRoutes_RecentIsPublicAndNotMissing(t *testing.T) {
	r := buildTestBolsaTrabajoRouter(t)

	req := httptest.NewRequest(http.MethodGet, "/api/v1/bolsa-trabajo/trabajo/recent", nil)
	w := httptest.NewRecorder()

	r.ServeHTTP(w, req)

	require.NotEqual(t, http.StatusNotFound, w.Code)
	assert.Equal(t, http.StatusOK, w.Code)
}

func TestBolsaTrabajoRoutes_AllRoutesRegistered(t *testing.T) {
	r := buildTestBolsaTrabajoRouter(t)

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/bolsa-trabajo/trabajo"},
		{http.MethodGet, "/api/v1/bolsa-trabajo/trabajo/activeJobs"},
		{http.MethodGet, "/api/v1/bolsa-trabajo/trabajo/by-empresa/00000000-0000-0000-0000-000000000001"},
		{http.MethodGet, "/api/v1/bolsa-trabajo/trabajo/recent"},
		{http.MethodPatch, "/api/v1/bolsa-trabajo/trabajo/switch/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/v1/bolsa-trabajo/postulacion/apply"},
		{http.MethodGet, "/api/v1/bolsa-trabajo/postulacion/by-trabajo-stats/00000000-0000-0000-0000-000000000001"},
		{http.MethodPatch, "/api/v1/bolsa-trabajo/postulacion/status/00000000-0000-0000-0000-000000000001"},
		{http.MethodPost, "/api/v1/bolsa-trabajo/evaluacion"},
		{http.MethodGet, "/api/v1/bolsa-trabajo/evaluacion/by-postulacion/00000000-0000-0000-0000-000000000001"},
	}

	for _, tc := range cases {
		t.Run(tc.method+" "+tc.path, func(t *testing.T) {
			req := httptest.NewRequest(tc.method, tc.path, nil)
			w := httptest.NewRecorder()
			r.ServeHTTP(w, req)
			assert.NotEqual(t, http.StatusNotFound, w.Code, "route not registered: %s %s", tc.method, tc.path)
			assert.NotEqual(t, http.StatusMethodNotAllowed, w.Code, "method not allowed: %s %s", tc.method, tc.path)
		})
	}
}
