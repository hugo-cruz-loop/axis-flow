package middleware

import (
	"context"
	"net/http"
)

// ContextKeyRole is the key used to store the authenticated user's role in context.
const ContextKeyRole contextKey = "role"

// RoleFromContext retrieves the role from the request context.
func RoleFromContext(ctx context.Context) (string, bool) {
	v, ok := ctx.Value(ContextKeyRole).(string)
	return v, ok
}

// RequireRoles returns middleware that allows the request only when the role in
// context matches one of the given allowed roles. It must run after JWTAuth.
func RequireRoles(roles ...string) func(http.Handler) http.Handler {
	allowed := make(map[string]struct{}, len(roles))
	for _, r := range roles {
		allowed[r] = struct{}{}
	}

	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			role, ok := RoleFromContext(r.Context())
			if !ok || role == "" {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			if _, permit := allowed[role]; !permit {
				http.Error(w, `{"error":"forbidden"}`, http.StatusForbidden)
				return
			}
			next.ServeHTTP(w, r)
		})
	}
}
