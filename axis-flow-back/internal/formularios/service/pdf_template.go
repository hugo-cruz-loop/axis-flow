// Package service — pdf_template.go: the safe HTML template + the
// ReportData view-model used by the PDF service.
//
// PR-6 (6.1). The spec's "Strict HTML Auto-Escaping" defense
// (docs/services/10_Formularios_Service_Spec/specification.md
// §Hardening the wkhtmltopdf PDF Generator, item 1) is implemented
// here as a Go html/template (NOT text/template) so every {{ .Field }}
// interpolation is HTML-escaped by default. The spec's
// --disable-javascript, --disable-local-file-access, and
// --disable-external-links flags are enforced at the Gotenberg
// Chromium layer; this template deliberately contains:
//
//   - NO <script> tags (so disabling JavaScript at Chromium is a
//     belt-and-suspenders backup, not the only line of defense)
//   - NO external <link> / <img src="http(s)://"> (all styling is
//     inline; no remote assets are loaded)
//   - NO inline event handlers (the html/template escape neutralizes
//     user-submitted <img onerror=...>, <a href="javascript:...">,
//     and file:// URLs at the template level)
//
// The template is parsed once at package init and the resulting
// *template.Template is reused on every render — this is the
// standard html/template pattern.
package service

import (
	"context"
	"fmt"
	"html/template"
	"io"
	"time"

	"axis-flow-back/internal/formularios"
)

// ---------------------------------------------------------------------------
// ReportData — the view-model the template renders. NOT a domain
// type; this is intentionally a thin, template-shaped struct so the
// template's expected field names are obvious and there is no risk
// of accidentally exposing a transport-layer json tag or PII field
// that the spec did not intend for PDF output.
// ---------------------------------------------------------------------------

// ReportData is the view-model that the report HTML template renders.
// All fields are template-friendly scalars or pre-shaped sub-views;
// there is no direct reference to a repository or HTTP-layer type.
type ReportData struct {
	// IniciadoID is the check-in this report belongs to.
	IniciadoID interface{ String() string }

	// EmpresaName is the human-readable tenant label. Optional —
	// when empty the template omits the "Empresa" row.
	EmpresaName string

	// ClienteName is the human-readable client label. Optional.
	ClienteName string

	// EmpleadoName is the human-readable employee label. Optional.
	EmpleadoName string

	// GeneratedAt is the timestamp the report was generated. The
	// caller is responsible for formatting it (the template does
	// NOT do any string conversion beyond the default).
	GeneratedAt string

	// Respuestas is the list of answers captured during the
	// check-in. The template iterates this list and renders one
	// <li> per respuesta; every field is auto-escaped.
	Respuestas []*formularios.Respuesta
}

// ---------------------------------------------------------------------------
// reportHTML — the actual template. Static, no external resources.
// ---------------------------------------------------------------------------

// reportHTML is the HTML5 template body. Inline CSS only. No
// <script>, no <link rel="stylesheet" href="http...">, no
// <img src="http...">. The data fields are referenced with {{ . }}
// which html/template escapes.
//
// The template is intentionally minimal (table-of-contents style
// "Reporte de Campo" header + one <li> per respuesta). The
// spec's report content is generic — the per-question rendering
// (Foto / Firma / Matriz) is out of scope for PR-6 and lands with
// the frontend's render pipeline.
const reportHTML = `<!DOCTYPE html>
<html lang="es">
<head>
<meta charset="utf-8">
<title>Reporte de Campo</title>
<style>
  body { font-family: Helvetica, Arial, sans-serif; color: #222; margin: 24px; }
  h1 { font-size: 20px; margin: 0 0 4px 0; }
  h2 { font-size: 14px; font-weight: normal; color: #666; margin: 0 0 16px 0; }
  .meta { font-size: 12px; color: #444; margin-bottom: 16px; }
  .meta span { display: block; margin-bottom: 2px; }
  ol { padding-left: 20px; }
  ol li { margin-bottom: 8px; font-size: 12px; }
  .qid { color: #999; font-size: 10px; }
  footer { margin-top: 24px; font-size: 10px; color: #999; }
</style>
</head>
<body>
  <h1>Reporte de Campo</h1>
  <h2>Iniciado {{ .IniciadoID }}</h2>
  <div class="meta">
    {{ if .EmpresaName }}<span><strong>Empresa:</strong> {{ .EmpresaName }}</span>{{ end }}
    {{ if .ClienteName }}<span><strong>Cliente:</strong> {{ .ClienteName }}</span>{{ end }}
    {{ if .EmpleadoName }}<span><strong>Empleado:</strong> {{ .EmpleadoName }}</span>{{ end }}
    <span><strong>Generado:</strong> {{ .GeneratedAt }}</span>
  </div>
  <h2>Respuestas</h2>
  <ol>
    {{ range .Respuestas }}
    <li>
      <span class="qid">{{ .PreguntaID }}</span>
      <div>{{ .RespuestaTexto }}</div>
    </li>
    {{ else }}
    <li><em>Sin respuestas registradas.</em></li>
    {{ end }}
  </ol>
  <footer>Documento generado automáticamente. Solo uso interno.</footer>
</body>
</html>`

// reportTemplate is the package-level, parsed-once, html/template
// (NOT text/template) used by every render. html/template
// auto-escapes every interpolation.
var reportTemplate = template.Must(template.New("reporte").Parse(reportHTML))

// ---------------------------------------------------------------------------
// RenderReporteHTML — the public render function. GREEN-step API
// surface that the test (and the future PDF service flow) calls.
// ---------------------------------------------------------------------------

// RenderReporteHTML executes the safe HTML template with data and
// writes the rendered bytes to w. Returns an error only on an I/O
// failure writing to w; the template itself does not fail at
// execution time (auto-escape happens in-line, no validation step).
//
// This function is package-level (not a method) so the TDD tests
// can call it directly without instantiating a service. The PDF
// service calls it from inside GenerateReporte before submitting
// the bytes to Gotenberg (see pdf_renderer.go for the transport).
func RenderReporteHTML(_ context.Context, w io.Writer, data ReportData) error {
	if w == nil {
		return fmt.Errorf("pdf: render: nil writer")
	}
	if err := reportTemplate.Execute(w, data); err != nil {
		return fmt.Errorf("pdf: render template: %w", err)
	}
	return nil
}

// Compile-time anchor — keeps the "time" import alive even if a
// future refactor drops the only use. The template currently does
// not call time-formatting helpers (the caller formats
// GeneratedAt), but the package reserves this anchor for a
// future "report header time-zone" requirement that the spec may
// add.
var _ = time.UTC
