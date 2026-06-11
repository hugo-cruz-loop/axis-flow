// Package service — PDFService: PDF report generation + S3 upload.
//
// PR-3 (Services) — task 3.4. Gherkin 3 (Generación asíncrona de reporte
// PDF con hardening de seguridad): the service fetches the parent
// chain, renders HTML to PDF via the PDFRenderer port, uploads to S3 via
// the ReportStorage port, and publishes ReporteGenerado. The actual
// wkhtmltopdf subprocess, Jinja2 templates, and S3 client land in
// PR-6 (PDF/S3 Hardening); the ports below are the seam.
//
// PR-4 AMEND (FIX 2): the GenerateReporte signature now returns
// (bytes, url, error). The handler streams the bytes to the client
// (no more "PDF stream placeholder" string with Content-Type
// application/pdf — that was corrupting downloads). The renderer
// already produced bytes; we just propagate them out of the
// service alongside the URL.
//
// PR-5 (5.3): the service is wired with the formularios
// *telemetry.Metrics struct (PDFRenderDuration / PDFRenderTotal /
// S3UploadDuration / S3UploadTotal / S3UploadBytes). The struct
// may be nil for callers that don't need metrics (legacy unit
// tests); the production wiring in main.go is required.
package service

import (
	"context"
	"fmt"
	"log/slog"
	"strings"
	"time"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"
	"axis-flow-back/internal/formularios/telemetry"

	"github.com/google/uuid"
)


// PDFRenderer is the port that turns an HTML template into PDF bytes.
// PR-6 will provide a wkhtmltopdf-backed implementation that enforces
// the security flags (--disable-javascript, --disable-local-file-access,
// --disable-external-links) and the Jinja2 auto-escape.
type PDFRenderer interface {
	Render(ctx context.Context, html string) ([]byte, error)
}

// ReportStorage is the port that uploads a generated PDF to durable
// storage. PR-6 will provide an S3-backed implementation.
type ReportStorage interface {
	Upload(ctx context.Context, key string, body []byte) (url string, err error)
}

// PDFService defines the use-case operation that turns a captured
// check-in into a downloadable PDF report URL.
type PDFService interface {
	// GenerateReporte orchestrates the full PDF flow for a check-in:
	//   1. Acquire the S3 lock (KeyReporteS3Lock) to dedupe concurrent
	//      jobs for the same iniciado.
	//   2. Add iniciadoID to KeyReportePending.
	//   3. Fetch respuestas, render HTML, upload to S3.
	//   4. Publish ReporteGenerado.
	//   5. Remove iniciadoID from KeyReportePending and release the lock.
	//
	// Returns (bytes, url, error):
	//   - bytes: the PDF binary returned by the renderer, propagated
	//     through the service so the HTTP handler can stream the
	//     real PDF to the client. PR-4 AMEND (FIX 2).
	//   - url:   the durable storage URL where the PDF was uploaded.
	//   - error: formularios.ErrNotFound if the iniciado has no
	//     respuestas (proxy for "unknown / foreign-tenant" — see
	//     Deviation #2); a wrapped error with a generic message for
	//     render / upload failures (no PII / vendor detail leaks).
	GenerateReporte(ctx context.Context, iniciadoID, empresaID uuid.UUID) ([]byte, string, error)
}

// pdfService is the concrete implementation.
type pdfService struct {
	evRepo   formularios.EventoRepository
	respRepo formularios.RespuestaRepository
	pub      events.EventPublisher
	cache    CacheInvalidator
	renderer PDFRenderer
	storage  ReportStorage
	locker   Locker
	metrics  *telemetry.Metrics
	// pool is the optional bounded worker pool (PR-6 6.3 —
	// Celery max(1, num_cores-1) equivalent). When non-nil,
	// the render+upload steps run inside pool.Execute; when nil,
	// the steps run inline (preserves the previous behavior for
	// legacy unit tests).
	pool *RenderPool
}

// NewPDFService constructs a PDFService. The renderer, storage, and
// locker are required (stubs in unit tests; real impls in PR-6).
// metrics and pool may be nil — the service is a no-op on the
// metric calls when nil, and skips the bounded-worker-pool
// wrapping when nil. The production wiring in main.go passes
// the real *telemetry.Metrics and a real *RenderPool (PR-6 6.3).
func NewPDFService(
	evRepo formularios.EventoRepository,
	respRepo formularios.RespuestaRepository,
	pub events.EventPublisher,
	cache CacheInvalidator,
	renderer PDFRenderer,
	storage ReportStorage,
	locker Locker,
	metrics *telemetry.Metrics,
	pool *RenderPool,
) PDFService {
	return &pdfService{
		evRepo:   evRepo,
		respRepo: respRepo,
		pub:      pub,
		cache:    cache,
		renderer: renderer,
		storage:  storage,
		locker:   locker,
		metrics:  metrics,
		pool:     pool,
	}
}

// ---------------------------------------------------------------------------
// GenerateReporte
// ---------------------------------------------------------------------------

func (s *pdfService) GenerateReporte(
	ctx context.Context,
	iniciadoID, empresaID uuid.UUID,
) ([]byte, string, error) {
	start := time.Now().UTC()

	// 1. Lock. Failure to acquire means a concurrent worker is already
	// generating the report; we propagate so the caller can retry.
	// PR-6 (6.3): the Locker port signature changed to take
	// uuid.UUID + duration (the locker builds the canonical
	// "formularios:reporte:lock:<uuid>" key internally).
	unlock, err := s.locker.Acquire(ctx, iniciadoID)
	if err != nil {
		s.recordPDFMetric(start, "error")
		s.logError(ctx, "GenerateReporte", "lock_acquire_failed", empresaID, err)
		return nil, "", fmt.Errorf("pdf: lock acquire failed")
	}
	defer func() { _ = unlock(ctx) }()

	// 2. Pending set. Best-effort; if the add fails we still continue
	// (the lock is held, the report will be generated). The remove
	// runs in defer to guarantee cleanup.
	// PR-6 (6.3): AddToPending / RemoveFromPending now take the
	// iniciadoID directly (the locker builds the
	// "formularios:reportes:pending" set key internally).
	_ = s.locker.AddToPending(ctx, iniciadoID)
	defer func() { _ = s.locker.RemoveFromPending(ctx, iniciadoID) }()

	// 3. Fetch respuestas. An iniciado with zero respuestas is treated
	// as "not found" — either the iniciadoID is unknown or it is in a
	// foreign tenant (IDOR is enforced at the handler layer; the
	// service uses this as a cheap proxy).
	respuestas, err := s.respRepo.ListByIniciado(ctx, iniciadoID)
	if err != nil {
		s.recordPDFMetric(start, "error")
		s.logError(ctx, "GenerateReporte", "load_respuestas_failed", empresaID, err)
		return nil, "", fmt.Errorf("pdf: load respuestas failed")
	}
	if len(respuestas) == 0 {
		s.recordPDFMetric(start, "error")
		s.logError(ctx, "GenerateReporte", "no_respuestas", empresaID, formularios.ErrNotFound)
		return nil, "", formularios.ErrNotFound
	}

	// Render the HTML template (stub in PR-3; real Jinja2 + auto-escape
	// in PR-6). The exact template is owned by the spec; PR-3 just
	// confirms the seam works end-to-end with deterministic stub HTML.
	//
	// PR-6 (6.3): when a *RenderPool is wired (production), the
	// render+upload steps run inside pool.Execute (the Celery
	// max(1, num_cores-1) equivalent). When pool is nil (legacy
	// unit tests), the steps run inline.
	html := buildReporteHTML(iniciadoID, respuestas)

	var pdfBytes []byte
	if s.pool != nil {
		err = s.pool.Execute(ctx, func() error {
			rendered, renderErr := s.renderer.Render(ctx, html)
			if renderErr != nil {
				return renderErr
			}
			pdfBytes = rendered
			return nil
		})
	} else {
		pdfBytes, err = s.renderer.Render(ctx, html)
	}
	if err != nil {
		// The underlying error message (e.g. "wkhtmltopdf: signal
		// killed", "exit status 1") is internal to the renderer and
		// can leak template paths / flag internals. We surface a
		// generic message to the caller; the structured logger (PR-5)
		// will persist the full error for SRE.
		s.recordPDFMetric(start, "error")
		s.logError(ctx, "GenerateReporte", "render_failed", empresaID, err)
		return nil, "", fmt.Errorf("pdf: render failed")
	}
	s.recordPDFMetric(start, "success")

	// Upload. Same convention: vendor detail (S3 error codes, internal
	// paths) is intentionally not surfaced.
	uploadStart := time.Now().UTC()
	key := buildReporteS3Key(iniciadoID)

	var url string
	if s.pool != nil {
		err = s.pool.Execute(ctx, func() error {
			uploaded, uploadErr := s.storage.Upload(ctx, key, pdfBytes)
			if uploadErr != nil {
				return uploadErr
			}
			url = uploaded
			return nil
		})
	} else {
		url, err = s.storage.Upload(ctx, key, pdfBytes)
	}
	if err != nil {
		s.recordS3Metric(uploadStart, "error", int64(len(pdfBytes)))
		s.logError(ctx, "GenerateReporte", "upload_failed", empresaID, err)
		return nil, "", fmt.Errorf("pdf: upload failed")
	}
	s.recordS3Metric(uploadStart, "success", int64(len(pdfBytes)))

	// 4. Publish + best-effort cache invalidation. PDF generation
	// does not invalidate a cached list (no list is keyed by iniciado
	// for the PDF flow), so the cache fn is a no-op here. The publish
	// step still runs through postWriteHook for symmetry / future
	// list invalidation.
	postWriteHook(ctx, s.pub, func(context.Context) error {
		return nil
	}, events.StreamReporteGenerado, map[string]any{
		"reporte_url":         url,
		"evento_iniciado_id": iniciadoID,
		"empresa_id":         empresaID,
	})

	// Structured success log: the brief's "level=info component=...
	// action=GenerateReporte duration_ms=... status=ok empresa_id=..."
	// shape. PII-free — the fields are IDs and durations only.
	slog.InfoContext(ctx, "formularios: GenerateReporte ok",
		slog.String("component", "formularios"),
		slog.String("action", "GenerateReporte"),
		slog.Int64("duration_ms", time.Since(start).Milliseconds()),
		slog.String("status", "ok"),
		slog.String("empresa_id", empresaID.String()),
		slog.String("evento_iniciado_id", iniciadoID.String()),
	)

	// PR-4 AMEND (FIX 2): propagate the rendered PDF bytes so the
	// HTTP handler can stream them to the client. The bytes have
	// already been uploaded to S3 (line above); returning them
	// here is the second use — the response body. PR-6 may replace
	// this with a redirect-to-S3 if the bytes are too large to
	// inline; for PR-4 the bytes are the small stub PDF.
	return pdfBytes, url, nil
}

// recordPDFMetric is a tiny nil-safe helper that observes the
// PDF render duration and increments the total counter. status is
// "success" or "error". No-op when metrics is nil (legacy tests).
func (s *pdfService) recordPDFMetric(start time.Time, status string) {
	if s.metrics == nil {
		return
	}
	elapsed := time.Since(start).Seconds()
	s.metrics.PDFRenderDuration.WithLabelValues(status).Observe(elapsed)
	s.metrics.PDFRenderTotal.WithLabelValues(status).Inc()
}

// recordS3Metric is a tiny nil-safe helper that observes the S3
// upload duration, increments the total counter, and records the
// byte count. status is "success" or "error". No-op when metrics
// is nil.
func (s *pdfService) recordS3Metric(start time.Time, status string, bytes int64) {
	if s.metrics == nil {
		return
	}
	elapsed := time.Since(start).Seconds()
	s.metrics.S3UploadDuration.WithLabelValues(status).Observe(elapsed)
	s.metrics.S3UploadTotal.WithLabelValues(status).Inc()
	if bytes > 0 {
		s.metrics.S3UploadBytes.WithLabelValues(status).Add(float64(bytes))
	}
}

// logError emits the structured error log the spec requires:
// "level=error component=formularios action=... err_class=...
// empresa_id=...". err_class is the sentinel/error name (e.g.
// ErrNotFound.Error()), NOT the message — the message is the
// surface for the client and must stay generic (no PII / vendor
// detail leaks). The full error is intentionally NOT included in
// the log; SREs get the structured class only and use the metric
// label for failure breakdowns.
func (s *pdfService) logError(ctx context.Context, action, errClass string, empresaID uuid.UUID, err error) {
	_ = err
	slog.ErrorContext(ctx, "formularios: "+action+" failed",
		slog.String("component", "formularios"),
		slog.String("action", action),
		slog.String("err_class", errClass),
		slog.String("empresa_id", empresaID.String()),
	)
}

// buildReporteHTML is a minimal HTML stub for PR-3. The real Jinja2
// template with auto-escape lands in PR-6. Kept in the service file
// (not extracted to a templater package) because it is intentionally
// trivial and changes infrequently.
func buildReporteHTML(iniciadoID uuid.UUID, respuestas []*formularios.Respuesta) string {
	var b strings.Builder
	fmt.Fprintf(&b, "<html><body><h1>Report %s</h1><ul>", iniciadoID)
	for _, r := range respuestas {
		// The real template will HTML-escape r.RespuestaTexto via
		// Jinja2 auto-escape; here we just print verbatim to prove the
		// seam receives the right input.
		fmt.Fprintf(&b, "<li>%s</li>", r.RespuestaTexto)
	}
	b.WriteString("</ul></body></html>")
	return b.String()
}

// buildReporteS3Key returns a stable, deterministic key so the same
// iniciado always lands on the same URL. PR-6 may add a content hash if
// idempotency on content changes is needed.
func buildReporteS3Key(iniciadoID uuid.UUID) string {
	return fmt.Sprintf("formularios/reports/%s.pdf", iniciadoID)
}
