// Package main — formularios_module.go: wires the Formularios
// module (repositories, services, handlers, telemetry, cross-domain
// consumer) and exposes the route registration function for main.go.
//
// PR-6 (6.4) implementation. The PR-5 placeholder ports
// (noopPDFRenderer / noopReportStorage / noopLocker) are
// removed; the production implementations are wired in:
//
//   - PDFRenderer       : service.NewGotenbergPDFRenderer(client)
//   - ReportStorage     : service.NewS3ReportStorage(client, bucket, prefix, timeout)
//   - Locker            : service.NewRedisLocker(rdb, ttl)
//   - RenderPool        : service.NewRenderPoolDefault() (max(1, NumCPU-1))
//   - GotenbergClient   : pdf/gotenberg.New(cfg.PDF.Endpoint, cfg.PDF.GotenbergEnabled)
//   - S3 client         : service.NewS3ClientFromConfig(ctx, formularios.S3 DTO)
//
// The "Gotenberg replaces wkhtmltopdf" decision is the user's
// PR-6 architectural decision (per the orchestrator brief);
// the Go html/template + safe transport contracts replace
// the spec's Jinja2 + wkhtmltopdf subprocess pair.
//
// The /api/v1/formularios route group is protected by
// JWTAuth + the role middleware stack, just like the rest of
// the app.
package main

import (
	"context"
	"net/http"

	"axis-flow-back/internal/config"
	formulariosHandler "axis-flow-back/internal/formularios/handler"
	formulariosEvents "axis-flow-back/internal/formularios/events"
	formulariosRepo "axis-flow-back/internal/formularios/repository"
	formulariosService "axis-flow-back/internal/formularios/service"
	formulariosTelemetry "axis-flow-back/internal/formularios/telemetry"
	"axis-flow-back/internal/middleware"
	"axis-flow-back/internal/pdf/gotenberg"

	"github.com/go-chi/chi/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/redis/go-redis/v9"
)

// formulariosModule holds the constructed handlers, services, and
// the cross-domain consumer for the Formularios module.
type formulariosModule struct {
	formH *formulariosHandler.FormularioHandler
	evH   *formulariosHandler.EventoHandler
	respH *formulariosHandler.RespuestaHandler

	// EventoCanceladoConsumer is the cross-domain Redis stream
	// consumer for the Asignacion → formularios "evento_cancelado"
	// flow. main.go calls Start(ctx) on it; the consumer exits
	// cleanly when the app context is cancelled.
	EventoCanceladoConsumer *formulariosEvents.EventoCanceladoConsumer
}

// formH / evH / respH expose the constructed handlers for the
// /api/v1/formularios route group in main.go.
func (m *formulariosModule) FormH() *formulariosHandler.FormularioHandler { return m.formH }
func (m *formulariosModule) EvH() *formulariosHandler.EventoHandler       { return m.evH }
func (m *formulariosModule) RespH() *formulariosHandler.RespuestaHandler { return m.respH }

// newFormulariosModule wires repositories → services → handlers for
// the formularios domain. The metrics registry is required so
// the PDF service can record the spec's observability counters.
//
// PR-6 (6.4) wiring: all three noop ports are replaced with the
// real implementations (Gotenberg HTTP, S3 with IAM-default
// credentials, Redis SET-NX-with-TTL locker). The render pool
// uses the spec's "Celery max(1, num_cores-1)" equivalent
// (max(1, runtime.NumCPU()-1)).
func newFormulariosModule(
	pool *pgxpool.Pool,
	rdb *redis.Client,
	cfg *config.Config,
	metricsReg prometheus.Registerer,
) *formulariosModule {
	// ── Telemetry ──────────────────────────────────────────────────────────
	metrics := formulariosTelemetry.NewMetrics(metricsReg)

	// ── Repositories ───────────────────────────────────────────────────────
	formRepo := formulariosRepo.NewPgxFormularioRepository(pool, rdb)
	preguntaRepo := formulariosRepo.NewPgxPreguntaRepository(pool)
	eventoRepo := formulariosRepo.NewPgxEventoRepository(pool, rdb)
	respuestaRepo := formulariosRepo.NewPgxRespuestaRepository(pool)

	// ── Cache invalidator ─────────────────────────────────────────────────
	cache := formulariosRepo.NewRedisFormulariosCacheInvalidator(rdb)

	// ── Event publisher ───────────────────────────────────────────────────
	publisher := formulariosEvents.NewRedisStreamPublisher(rdb)

	// ── PDF transport (Gotenberg HTTP) ────────────────────────────────────
	// Reuses the shared internal/pdf/gotenberg client (refactored
	// from internal/cursos/gotenberg in PR-6 step 6.0). When
	// cfg.PDF.GotenbergEnabled is false the client is a mock
	// (useful for dev / test environments without a running
	// Gotenberg instance).
	pdfClient := gotenberg.New(cfg.PDF.Endpoint, cfg.PDF.GotenbergEnabled)
	pdfRenderer := formulariosService.NewGotenbergPDFRenderer(pdfClient)

	// ── S3 transport ──────────────────────────────────────────────────────
	// IAM-default credential chain in production; static keys for
	// local dev / MinIO. The S3 DTO is a thin mirror of
	// config.FormulariosS3Config — the service package cannot
	// import the config package without a cycle.
	s3Client, err := formulariosService.NewS3ClientFromConfig(context.Background(), formulariosService.FormulariosS3ConfigForTest{
		Region:          cfg.Formularios.S3.Region,
		AccessKeyID:     cfg.Formularios.S3.AccessKeyID,
		SecretAccessKey: cfg.Formularios.S3.SecretAccessKey,
	})
	if err != nil {
		// The helper returns an error only when the static
		// credentials are malformed (one set, not the other).
		// In production with IAM role, this branch is
		// unreachable. Log and continue with a nil client;
		// the service will fail loudly on the first Upload.
		panic("formularios: invalid S3 credentials: " + err.Error())
	}
	reportStorage := formulariosService.NewS3ReportStorage(
		s3Client,
		cfg.Formularios.S3.Bucket,
		"reportes/",
		cfg.Formularios.S3.UploadTimeout,
	)

	// ── Locker (Redis SET-NX-with-TTL) ────────────────────────────────────
	// TTL is configurable via FORMULARIOS_LOCK_TTL (default 5m).
	locker := formulariosService.NewRedisLocker(rdb, cfg.Formularios.LockTTL)

	// ── Render pool (Celery equivalent) ───────────────────────────────────
	// max(1, NumCPU-1) — mirrors the spec's "Celery
	// max(1, num_cores-1)" concurrency cap.
	renderPool := formulariosService.NewRenderPoolDefault()

	// ── Services ───────────────────────────────────────────────────────────
	formSvc := formulariosService.NewFormularioService(formRepo, preguntaRepo, publisher, cache)
	eventoSvc := formulariosService.NewEventoService(eventoRepo, publisher, cache)
	respSvc := formulariosService.NewRespuestaService(respuestaRepo, publisher, cache)
	pdfSvc := formulariosService.NewPDFService(
		eventoRepo, respuestaRepo, publisher, cache,
		pdfRenderer, reportStorage, locker, metrics, renderPool,
	)

	// ── Handlers ───────────────────────────────────────────────────────────
	formH := formulariosHandler.NewFormularioHandler(formSvc)
	evH := formulariosHandler.NewEventoHandler(eventoSvc)
	respH := formulariosHandler.NewRespuestaHandler(respSvc, pdfSvc)

	// ── Cross-domain consumer (Asignacion → formularios) ──────────────────
	canceladoConsumer := formulariosEvents.NewEventoCanceladoConsumer(rdb, eventoSvc)

	return &formulariosModule{
		formH:                  formH,
		evH:                    evH,
		respH:                  respH,
		EventoCanceladoConsumer: canceladoConsumer,
	}
}

// registerFormulariosRoutesWithAuth mounts the /api/v1/formularios
// route group with the bound JWTAuth middleware. main.go calls
// this from inside a router group that already has JWTAuth applied,
// so the route-level middleware is a no-op.
func registerFormulariosRoutesWithAuth(
	r chi.Router,
	jwtAuth func(http.Handler) http.Handler,
	formH *formulariosHandler.FormularioHandler,
	evH *formulariosHandler.EventoHandler,
	respH *formulariosHandler.RespuestaHandler,
) {
	registerFormulariosRoutes(r, jwtAuth, middleware.RequireRoles, formH, evH, respH)
}
