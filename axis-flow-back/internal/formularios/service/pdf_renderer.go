// Package service — pdf_renderer.go: the GotenbergPDFRenderer
// transport that satisfies the service.PDFRenderer port using the
// shared internal/pdf/gotenberg client (PR-6 step 6.0 moved the
// package from internal/cursos/gotenberg to its own neutral home).
//
// PR-6 (6.1) implementation. The renderer is a thin transport —
// the auto-escape guarantee lives in pdf_template.go's
// html/template; the renderer just submits whatever HTML the
// service produces to Gotenberg and returns the PDF bytes
// unchanged. The spec's "no PII / no vendor detail" surfacing is
// enforced here: any 4xx/5xx response is wrapped as
// ErrPDFRenderInternal with a generic message; the underlying
// status code / body is logged via slog at the service layer (not
// here — the renderer stays transport-agnostic).
package service

import (
	"context"
	"errors"
	"fmt"

	"axis-flow-back/internal/pdf/gotenberg"
)

// ErrPDFRenderInternal is the surfaced error class for any
// Gotenberg failure. Callers (the PDF service) compare against
// this sentinel to record a "error" metric and a generic error
// log. The underlying error is intentionally NOT wrapped (no %w)
// so the user-facing message is always the same generic prefix.
var ErrPDFRenderInternal = errors.New("pdf: render failed")

// GotenbergPDFRenderer satisfies the PDFRenderer port by
// delegating to a GotenbergClient. The client can be either the
// real HTTP-backed client (production) or the mock client (dev /
// test environments without a running Gotenberg instance).
type GotenbergPDFRenderer struct {
	client gotenberg.GotenbergClient
}

// NewGotenbergPDFRenderer constructs a renderer backed by the
// given Gotenberg client. The client is the dependency injection
// seam: tests use a mock, production uses the real HTTP client.
func NewGotenbergPDFRenderer(client gotenberg.GotenbergClient) *GotenbergPDFRenderer {
	return &GotenbergPDFRenderer{client: client}
}

// Render submits html to Gotenberg and returns the resulting PDF
// bytes. The html MUST already be auto-escaped (the service runs
// it through html/template before calling here). The renderer
// does NOT re-escape — its only job is the HTTP transport.
//
// Error surfacing: any non-2xx response, network error, or
// context cancellation is wrapped as ErrPDFRenderInternal with a
// generic "pdf: render failed" prefix. The raw status code and
// body are NOT included in the returned error to honor the
// spec's "no PII / no vendor detail" convention. The structured
// logger at the service layer captures the full error for SRE.
func (r *GotenbergPDFRenderer) Render(ctx context.Context, html string) ([]byte, error) {
	if r.client == nil {
		return nil, fmt.Errorf("%w: no client", ErrPDFRenderInternal)
	}
	pdf, err := r.client.GeneratePDF(ctx, html)
	if err != nil {
		// Wrap as the generic internal error. We deliberately do
		// NOT use %w on the underlying client error — the
		// "gotenberg: ..." string in the client error is not
		// PII, but the rest of the call chain (slog) needs a
		// single stable error class to match on.
		return nil, fmt.Errorf("%w", ErrPDFRenderInternal)
	}
	if len(pdf) == 0 {
		return nil, fmt.Errorf("%w: empty response", ErrPDFRenderInternal)
	}
	return pdf, nil
}
