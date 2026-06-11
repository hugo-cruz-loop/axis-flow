// Package main — formularios_routes_test.go: route matrix test for
// the formularios module.
//
// PR-4 (REST/HTTP) — task 4.4. Mirrors the structure of
// cmd/server/atencion_routes_test.go (PR-4 of
// 09_AtencionSeguimiento_Service_Spec). The test harness:
//
//  1. Wires the real chi router + the real registerFormulariosRoutes
//     function with stub services.
//  2. Walks the 8-route matrix asserting each route is REGISTERED
//     (responds with something other than 404 when the JWT/auth
//     middleware is bypassed).
//  3. Walks the route matrix with a "always-unauthorized" middleware
//     and asserts each route requires auth (401).
//  4. Walks the route matrix with a "always-forbidden" role guard
//     and asserts role-restricted routes return 403.
package main

import (
	"bytes"
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"

	formulariosHandler "axis-flow-back/internal/formularios/handler"
	"axis-flow-back/internal/middleware"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
)

// buildTestFormulariosRouter wires a chi router with the formularios
// routes mounted. The JWT and role middleware are passed in so the
// test can control the auth outcome.
func buildTestFormulariosRouter(
	jwtAuth func(http.Handler) http.Handler,
	requireRoles func(...string) func(http.Handler) http.Handler,
) chi.Router {
	r := chi.NewRouter()
	formH := formulariosHandler.NewFormularioHandler(&stubFormularioSvc{})
	evH := formulariosHandler.NewEventoHandler(&stubEventoSvc{})
	respH := formulariosHandler.NewRespuestaHandler(&stubRespuestaSvc{}, &stubPDFSvc{})
	registerFormulariosRoutes(r, jwtAuth, requireRoles, formH, evH, respH)
	return r
}

// alwaysUnauthorized is a stand-in JWT middleware that rejects every
// request with 401 (mirrors the 09 test harness).
func alwaysUnauthorized(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		http.Error(w, `{"error":"unauthorized"}`, http.StatusUnauthorized)
	})
}

// alwaysAllowJWT is a stand-in JWT middleware that admits every
// request (lets the test reach the role-guard layer).
func alwaysAllowJWT(next http.Handler) http.Handler { return next }

// allowOnlyRoles is a helper that builds a requireRoles middleware
// admitting only the supplied role names.
func allowOnlyRoles(allowed ...string) func(...string) func(http.Handler) http.Handler {
	return func(roles ...string) func(http.Handler) http.Handler {
		set := map[string]struct{}{}
		for _, r := range roles {
			set[r] = struct{}{}
		}
		return func(next http.Handler) http.Handler {
			return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				role := r.Header.Get("X-Test-Role")
				if _, ok := set[role]; !ok {
					http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
					return
				}
				next.ServeHTTP(w, r)
			})
		}
	}
}

// injectTenantJWT is a stand-in JWT middleware that injects the
// test tenant into the request context so the handler's
// extractTenantID succeeds. Mirrors the 09 atencion_stubs_test.go
// pattern (which doesn't have a similar helper because the 09
// happy-path test relies on integration tests).
func injectTenantJWT(tenant string) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			ctx := r.Context()
			ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenant)
			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// contextWithTenant is an alias kept for clarity at the call site.
func contextWithTenant(ctx context.Context, tenant string) context.Context {
	return context.WithValue(ctx, middleware.ContextKeyTenantID, tenant)
}

// ---------------------------------------------------------------------------
// 401 matrix — every route must enforce JWT auth.
// ---------------------------------------------------------------------------

func TestFormulariosRoutes_AllRoutesReturn401WithoutJWT(t *testing.T) {
	r := buildTestFormulariosRouter(alwaysUnauthorized, allowOnlyRoles("Admin", "Manager"))

	cases := []struct {
		method string
		path   string
	}{
		{http.MethodPost, "/api/v1/formularios/formulario"},
		{http.MethodGet, "/api/v1/formularios/formulario/byempresa/" + uuid.New().String()},
		{http.MethodPost, "/api/v1/formularios/pregunta"},
		{http.MethodPost, "/api/v1/formularios/evento"},
		{http.MethodPost, "/api/v1/formularios/evento_iniciado"},
		{http.MethodGet, "/api/v1/formularios/evento/byEmpId/" + uuid.New().String() + "/" + uuid.New().String()},
		{http.MethodPost, "/api/v1/formularios/respuesta"},
		{http.MethodGet, "/api/v1/formularios/respuesta/reporte/pregunta/" + uuid.New().String() + "/pdf"},
	}

	for _, tc := range cases {
		var body *bytes.Buffer
		if tc.method == http.MethodPost {
			body = bytes.NewBufferString(`{}`)
		} else {
			body = bytes.NewBuffer(nil)
		}
		req := httptest.NewRequest(tc.method, tc.path, body)
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equalf(t, http.StatusUnauthorized, w.Code,
			"%s %s must return 401 without JWT, got %d", tc.method, tc.path, w.Code)
	}
}

// ---------------------------------------------------------------------------
// 403 matrix — role-restricted routes must reject wrong role.
// ---------------------------------------------------------------------------

func TestFormulariosRoutes_AdminOnlyRoutes_Return403ForEmpleado(t *testing.T) {
	r := buildTestFormulariosRouter(alwaysAllowJWT, allowOnlyRoles("Admin", "Manager"))

	// Routes that require Admin or Manager. Calling as Empleado →
	// 403. We pass a body so the handler doesn't short-circuit on
	// body validation before the role check.
	body, _ := json.Marshal(map[string]any{
		"empresa_id": uuid.New(),
		"nombre":     "X",
		"activo":     true,
	})

	adminOnlyPaths := []struct {
		method, path string
	}{
		{http.MethodPost, "/api/v1/formularios/formulario"},
		{http.MethodPost, "/api/v1/formularios/pregunta"},
		{http.MethodPost, "/api/v1/formularios/evento"},
	}
	for _, tc := range adminOnlyPaths {
		req := httptest.NewRequest(tc.method, tc.path, bytes.NewReader(body))
		req.Header.Set("X-Test-Role", "Empleado")
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.Equalf(t, http.StatusForbidden, w.Code,
			"%s %s must return 403 for Empleado, got %d", tc.method, tc.path, w.Code)
	}
}

// ---------------------------------------------------------------------------
// 200 / 201 matrix — happy paths reach the handler (stub service returns
// a benign no-op). 422 is acceptable when the stub body is rejected by
// handler-level validation (e.g. empty nombre).
// ---------------------------------------------------------------------------

func TestFormulariosRoutes_AllRoutesAreRegistered(t *testing.T) {
	r := buildTestFormulariosRouter(alwaysAllowJWT, allowOnlyRoles("Admin", "Manager", "Empleado"))

	cases := []struct {
		method, path, role string
		body               string
	}{
		{http.MethodPost, "/api/v1/formularios/formulario", "Admin", `{"empresa_id":"` + uuid.New().String() + `","nombre":"X","activo":true}`},
		{http.MethodGet, "/api/v1/formularios/formulario/byempresa/" + uuid.New().String(), "Admin", ""},
		{http.MethodPost, "/api/v1/formularios/pregunta", "Admin", `{"formulario_id":"` + uuid.New().String() + `","orden":1,"texto_pregunta":"x","tipo_pregunta":1,"obligatoria":true}`},
		{http.MethodPost, "/api/v1/formularios/evento", "Manager", `{"empresa_id":"` + uuid.New().String() + `","cliente_id":"` + uuid.New().String() + `","nombre":"X","fecha_programada":"2026-06-10T22:00:00Z"}`},
		{http.MethodPost, "/api/v1/formularios/evento_iniciado", "Empleado", `{"evento_id":"` + uuid.New().String() + `","empleado_id":99}`},
		{http.MethodGet, "/api/v1/formularios/evento/byEmpId/" + uuid.New().String() + "/" + uuid.New().String(), "Empleado", ""},
		{http.MethodPost, "/api/v1/formularios/respuesta", "Empleado", `{"evento_iniciado_id":"` + uuid.New().String() + `","formulario_id":"` + uuid.New().String() + `","pregunta_id":"` + uuid.New().String() + `","respuesta_lista":{}}`},
		{http.MethodGet, "/api/v1/formularios/respuesta/reporte/pregunta/" + uuid.New().String() + "/pdf", "Empleado", ""},
	}

	for _, tc := range cases {
		var reqBody *bytes.Buffer
		if tc.body == "" {
			reqBody = bytes.NewBuffer(nil)
		} else {
			reqBody = bytes.NewBufferString(tc.body)
		}
		req := httptest.NewRequest(tc.method, tc.path, reqBody)
		req.Header.Set("X-Test-Role", tc.role)
		if tc.method == http.MethodPost {
			req.Header.Set("Content-Type", "application/json")
		}
		w := httptest.NewRecorder()
		r.ServeHTTP(w, req)
		assert.NotEqualf(t, http.StatusNotFound, w.Code,
			"%s %s is not registered (got 404)", tc.method, tc.path)
		// The handler enforces a tenant-from-JWT check that returns
		// 401 when the stub middleware doesn't inject claims. This
		// 401 is the CORRECT response — the route IS registered, the
		// tenant extractor is just unsatisfied. PR-5 will wire a
		// tenant-injecting stub middleware to exercise the happy
		// 200/201 paths.
	}
}

// TestFormulariosRoutes_HappyPathPOSTFormulario_Returns201 is a smoke
// test for the wired route + stub service composition. It catches
// regressions in the JSON DTO conversion that would otherwise only
// surface in the integration test in PR-8.
func TestFormulariosRoutes_HappyPathPOSTFormulario_Returns201(t *testing.T) {
	tenantID := uuid.New()
	r := buildTestFormulariosRouter(injectTenantJWT(tenantID.String()), allowOnlyRoles("Admin", "Manager"))

	body, _ := json.Marshal(map[string]any{
		"empresa_id": tenantID,
		"nombre":     "Control Higienico",
		"activo":     true,
	})
	req := httptest.NewRequest(http.MethodPost, "/api/v1/formularios/formulario", bytes.NewReader(body))
	req.Header.Set("Content-Type", "application/json")
	req.Header.Set("X-Test-Role", "Admin")
	w := httptest.NewRecorder()
	r.ServeHTTP(w, req)

	assert.Equal(t, http.StatusCreated, w.Code, "happy POST /formulario → 201, body: %s", w.Body.String())
	var env map[string]any
	_ = json.Unmarshal(w.Body.Bytes(), &env)
	assert.Equal(t, true, env["success"])
}
