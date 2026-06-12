// Package main registers the ReportBro HTTP surface on the chi router.
//
// Route map:
//
//	POST /api/v1/report/run/{type}  — JWT-protected; generate report and return signed URL
//	GET  /api/v1/report/run/{key}   — public; HMAC sig is the auth mechanism
package main

import (
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/report/handler"
	"axis-flow-back/internal/service"

	"github.com/go-chi/chi/v5"
)

// registerReportRoutes mounts the two report endpoints on the supplied chi router.
// The POST endpoint requires a valid JWT (via JWTAuth middleware).
// The GET endpoint has no JWT — the HMAC signature embedded in the URL is the auth.
func registerReportRoutes(r chi.Router, h *handler.ReportHandler, authSvc *service.AuthService) {
	// POST requires JWT — signed URL building needs the caller's userID.
	r.Group(func(r chi.Router) {
		r.Use(middleware.JWTAuth(authSvc))
		r.Post("/api/v1/report/run/{type}", h.RunReport)
	})

	// GET has no JWT — HMAC sig carried in query params is the auth mechanism.
	r.Get("/api/v1/report/run/{key}", h.DownloadReport)
}
