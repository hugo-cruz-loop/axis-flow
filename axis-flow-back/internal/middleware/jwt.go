// Package middleware provides HTTP middleware for the identity service.
package middleware

import (
	"context"
	"net/http"
	"strings"

	"axis-flow-back/internal/service"
)

type contextKey string

// ContextKeyUserID is the key used to store the authenticated user ID in request context.
const ContextKeyUserID contextKey = "userID"

// ContextKeyTenantID is the key used to store the authenticated tenant ID in request context.
const ContextKeyTenantID contextKey = "tenantID"

// ContextKeyEmail is the key used to store the authenticated user email in request context.
const ContextKeyEmail contextKey = "email"

// ContextKeyEmpleadoID is the key used to store the authenticated employee ID in request context.
const ContextKeyEmpleadoID contextKey = "empleadoID"

// JWTAuth returns a middleware that validates the Authorization: Bearer <token> header.
// On success, it injects the user ID, tenant ID, and PR-5 empleado ID
// into the request context. On failure, it responds with 401 Unauthorized.
//
// PR-5 (5.0): the access token's empleado_id claim is written under
// ContextKeyEmpleadoID. Tokens without the claim (older issuer, or
// the user has no linked empleado) result in int64(0) — the
// formularios extractEmpleadoID helper rejects 0, but the rest of
// the system treats 0 the same as missing. The key is always
// written, never skipped, so the handler can distinguish
// "key not present" from "key present with value 0" if needed
// (today no handler does).
func JWTAuth(authSvc *service.AuthService) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			header := r.Header.Get("Authorization")
			if header == "" || !strings.HasPrefix(header, "Bearer ") {
				http.Error(w, "missing or malformed authorization header", http.StatusUnauthorized)
				return
			}

			tokenStr := strings.TrimPrefix(header, "Bearer ")
			claims, err := authSvc.ParseAccessToken(tokenStr)
			if err != nil {
				http.Error(w, "invalid or expired token", http.StatusUnauthorized)
				return
			}

			ctx := context.WithValue(r.Context(), ContextKeyUserID, claims.UserID)
			ctx = context.WithValue(ctx, ContextKeyTenantID, claims.TenantID)
			ctx = context.WithValue(ctx, ContextKeyEmail, claims.Email)
			ctx = context.WithValue(ctx, ContextKeyRole, claims.Role)
			ctx = context.WithValue(ctx, ContextKeyEmpleadoID, claims.EmpleadoID)
			SetLogIdentity(ctx, claims.UserID, claims.Role)

			next.ServeHTTP(w, r.WithContext(ctx))
		})
	}
}

// UserIDFromContext retrieves the authenticated user ID from context.
func UserIDFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyUserID).(string)
	return v, ok
}
