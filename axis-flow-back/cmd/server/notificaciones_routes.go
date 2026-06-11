package main

import (
	"net/http"

	notifhandler "axis-flow-back/internal/notificaciones/handler"
	"axis-flow-back/internal/notificaciones/ws"

	"github.com/go-chi/chi/v5"
)

// registerNotificacionesRoutes mounts all Notificaciones REST and WebSocket routes.
//
// Route matrix:
//
//	POST  /api/v1/notificaciones/recuperar-password                  (public) → h.RecuperarPassword
//	POST  /api/v1/notificaciones/bienvenida                          JWTAuth  → h.Bienvenida
//	GET   /api/v1/notificaciones/usuario/{userId}                    JWTAuth  → h.ListPushHistory
//	PATCH /api/v1/notificaciones/{id}/estatus                        JWTAuth  → h.UpdateEstatus
//	GET   /api/v1/notificaciones/usuario/{userId}/no-leidas/count    JWTAuth  → h.CountUnread
//	GET   /ws/notificaciones/                                        token qp → wsHandler.ServeWS
func registerNotificacionesRoutes(
	r chi.Router,
	h *notifhandler.NotificationHandler,
	wsHandler *ws.Handler,
	jwtAuth func(http.Handler) http.Handler,
) {
	// Public — no authentication required.
	r.Post("/api/v1/notificaciones/recuperar-password", h.RecuperarPassword)

	// Protected — JWT required.
	r.Group(func(r chi.Router) {
		r.Use(jwtAuth)
		r.Post("/api/v1/notificaciones/bienvenida", h.Bienvenida)
		r.Get("/api/v1/notificaciones/usuario/{userId}", h.ListPushHistory)
		r.Patch("/api/v1/notificaciones/{id}/estatus", h.UpdateEstatus)
		r.Get("/api/v1/notificaciones/usuario/{userId}/no-leidas/count", h.CountUnread)
	})

	// WebSocket upgrade — JWT via ?token= query param (browser WS API limitation).
	r.Get("/ws/notificaciones/", wsHandler.ServeWS)
}
