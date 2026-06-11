// Package handler_test covers the HTTP handlers and helpers for the
// Formularios module.
//
// PR-4 (REST/HTTP) — task 4.1 helpers. Mirrors the structure of
// internal/atencionseguimiento/handler/queja_handler_test.go (PR-4 of
// 09_AtencionSeguimiento_Service_Spec). Each test uses httptest.NewRecorder
// + http.NewRequest with JWT claims injected via context.WithValue so the
// handler is exercised without a real HTTP server, JWT signer, or DB.
package handler_test

import (
	"context"
	"net/http"

	"axis-flow-back/internal/middleware"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// JWT-context injection helper shared by every formularios handler test.
// ---------------------------------------------------------------------------

// injectFormulariosCtx puts the four JWT claim values used by the formularios
// handlers into the request context. Mirrors `injectQuejaCtx` in
// internal/atencionseguimiento/handler/queja_handler_test.go.
//
// The current middleware.JWTAuth populates ContextKeyUserID, ContextKeyTenantID,
// ContextKeyEmail, and ContextKeyRole from the access-token claims. It does NOT
// yet populate ContextKeyEmpleadoID (the production JWT schema does not include
// an empleado_id claim). The formularios handler still reads EmpleadoID from
// the context for /evento_iniciado, so the production wiring for that
// claim is a TODO that PR-4 documents (Deviation #3) — the tests inject it
// here to keep the handler behaviour consistent.
func injectFormulariosCtx(r *http.Request, tenantID, userID string, empleadoID int64, role string) *http.Request {
	ctx := r.Context()
	ctx = context.WithValue(ctx, middleware.ContextKeyTenantID, tenantID)
	ctx = context.WithValue(ctx, middleware.ContextKeyUserID, userID)
	ctx = context.WithValue(ctx, middleware.ContextKeyEmpleadoID, empleadoID)
	ctx = context.WithValue(ctx, middleware.ContextKeyRole, role)
	return r.WithContext(ctx)
}

// uuidPtr returns &u for inline DTOs.
func uuidPtr(u uuid.UUID) *uuid.UUID { return &u }
