// Package service — PDFService: PDF report generation + S3 upload.
//
// PR-3 (Services) — task 3.4. Gherkin 3 (Generación asíncrona de reporte
// PDF con hardening de seguridad): the service fetches the parent
// chain, renders HTML to PDF via the PDFRenderer port, uploads to S3 via
// the ReportStorage port, and publishes ReporteGenerado. The actual
// wkhtmltopdf subprocess, Jinja2 templates, and S3 client land in
// PR-6 (PDF/S3 Hardening); the ports below are the seam.
package service

import (
	"context"
	"fmt"
	"strings"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/events"

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
	// Errors: formularios.ErrNotFound if the iniciado has no respuestas
	// (proxy for "unknown / foreign-tenant" — see Deviation #2); a
	// wrapped error with a generic message for render / upload
	// failures (no PII / vendor detail leaks).
	GenerateReporte(ctx context.Context, iniciadoID, empresaID uuid.UUID) (string, error)
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
}

// reportLockTTL is the TTL for the S3 report lock. Long enough to cover
// a typical PDF render (10s per the spec's WKHTMLTOPDF_TIMEOUT_SECONDS)
// with headroom for a slow VPS, short enough that a crashed worker does
// not block retries for hours.
const reportLockTTLSeconds = 300

// NewPDFService constructs a PDFService. The renderer, storage, and
// locker are required (stubs in unit tests; real impls in PR-6).
func NewPDFService(
	evRepo formularios.EventoRepository,
	respRepo formularios.RespuestaRepository,
	pub events.EventPublisher,
	cache CacheInvalidator,
	renderer PDFRenderer,
	storage ReportStorage,
	locker Locker,
) PDFService {
	return &pdfService{
		evRepo:   evRepo,
		respRepo: respRepo,
		pub:      pub,
		cache:    cache,
		renderer: renderer,
		storage:  storage,
		locker:   locker,
	}
}

// ---------------------------------------------------------------------------
// GenerateReporte
// ---------------------------------------------------------------------------

func (s *pdfService) GenerateReporte(
	ctx context.Context,
	iniciadoID, empresaID uuid.UUID,
) (string, error) {
	// 1. Lock. Failure to acquire means a concurrent worker is already
	// generating the report; we propagate so the caller can retry.
	lockKey := fmt.Sprintf(formularios.KeyReporteS3Lock, iniciadoID)
	unlock, err := s.locker.Acquire(ctx, lockKey, reportLockTTLSeconds)
	if err != nil {
		return "", fmt.Errorf("pdf: lock acquire failed")
	}
	defer func() { _ = unlock(ctx) }()

	// 2. Pending set. Best-effort; if the add fails we still continue
	// (the lock is held, the report will be generated). The remove
	// runs in defer to guarantee cleanup.
	pendingKey := formularios.KeyReportePending
	_ = s.locker.AddToPending(ctx, pendingKey, iniciadoID.String())
	defer func() { _ = s.locker.RemoveFromPending(ctx, pendingKey, iniciadoID.String()) }()

	// 3. Fetch respuestas. An iniciado with zero respuestas is treated
	// as "not found" — either the iniciadoID is unknown or it is in a
	// foreign tenant (IDOR is enforced at the handler layer; the
	// service uses this as a cheap proxy).
	respuestas, err := s.respRepo.ListByIniciado(ctx, iniciadoID)
	if err != nil {
		return "", fmt.Errorf("pdf: load respuestas failed")
	}
	if len(respuestas) == 0 {
		return "", formularios.ErrNotFound
	}

	// Render the HTML template (stub in PR-3; real Jinja2 + auto-escape
	// in PR-6). The exact template is owned by the spec; PR-3 just
	// confirms the seam works end-to-end with deterministic stub HTML.
	html := buildReporteHTML(iniciadoID, respuestas)
	pdfBytes, err := s.renderer.Render(ctx, html)
	if err != nil {
		// The underlying error message (e.g. "wkhtmltopdf: signal
		// killed", "exit status 1") is internal to the renderer and
		// can leak template paths / flag internals. We surface a
		// generic message to the caller; the structured logger (PR-5)
		// will persist the full error for SRE.
		return "", fmt.Errorf("pdf: render failed")
	}

	// Upload. Same convention: vendor detail (S3 error codes, internal
	// paths) is intentionally not surfaced.
	key := buildReporteS3Key(iniciadoID)
	url, err := s.storage.Upload(ctx, key, pdfBytes)
	if err != nil {
		return "", fmt.Errorf("pdf: upload failed")
	}

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

	return url, nil
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
