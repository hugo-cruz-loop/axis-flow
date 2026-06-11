package handler_test

import (
	"context"
	"net/http"

	"github.com/go-chi/chi/v5"
)

// requestWithPathParam injects a chi URL parameter into the request context.
// This avoids spinning up a full chi router in unit tests.
func requestWithPathParam(r *http.Request, key, value string) *http.Request {
	rctx := chi.NewRouteContext()
	rctx.URLParams.Add(key, value)
	return r.WithContext(context.WithValue(r.Context(), chi.RouteCtxKey, rctx))
}
