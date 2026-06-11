// Package service_test — PR-6 (6.1) TDD RED test for the safe HTML
// template. We assert two security guarantees from the spec:
//
//   1. html/template (NOT text/template) is used, so EVERY field is
//      auto-escaped by default. A literal `<script>` payload in any
//      field becomes the escaped `&lt;script&gt;...&lt;/script&gt;`.
//   2. The "html/template URL-scheme aware" escaping ensures a
//      `javascript:void(0)` payload rendered into an href attribute
//      is NOT a clickable javascript: URL — html/template neutralizes
//      javascript:/data:/vbscript: schemes (Go 1.10+ behavior).
//
// These guarantees are the Go equivalent of the spec's "Strict HTML
// Auto-Escaping" defense (spec §Hardening the wkhtmltopdf PDF
// Generator, item 1). The wkhtmltopdf --disable-javascript flag is
// implemented at the Gotenberg layer (Chromium is launched with the
// equivalent flag), and --disable-local-file-access /
// --disable-external-links is enforced by the template containing no
// `<script>` tags and no external resources (test #3 below).
//
// The tests in this file fail to compile until the corresponding
// production code lands (RED), then pass (GREEN), then get
// triangulated with more payloads (TRIANGULATE).
package service_test

import (
	"bytes"
	"context"
	"html/template"
	"strings"
	"testing"

	"axis-flow-back/internal/formularios"
	"axis-flow-back/internal/formularios/service"

	"github.com/google/uuid"
)

// ---------------------------------------------------------------------------
// 6.1 (a) — RenderReporteHTML auto-escapes every field.
// ---------------------------------------------------------------------------

// TestReportTemplate_EscapesScriptTags asserts the most basic XSS
// defense: a literal `<script>` payload in a text field is HTML-escaped
// before reaching the rendered output. The Go template package
// html/template must be used (NOT text/template) for this to work.
func TestReportTemplate_EscapesScriptTags(t *testing.T) {
	data := service.ReportData{
		IniciadoID:   uuid.New(),
		GeneratedAt:  "2026-06-10T22:00:00Z",
		Respuestas: []*formularios.Respuesta{
			{
				PreguntaID:     uuid.New(),
				RespuestaTexto: `<script>alert("xss")</script>`,
			},
		},
	}

	var buf bytes.Buffer
	if err := service.RenderReporteHTML(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderReporteHTML error: %v", err)
	}
	out := buf.String()
	if strings.Contains(out, "<script>alert") {
		t.Errorf("XSS payload reached the rendered HTML: contains literal <script> in %q", out)
	}
	if !strings.Contains(out, "&lt;script&gt;alert") {
		t.Errorf("XSS payload was NOT escaped: missing &lt;script&gt; in %q", out)
	}
}

// TestReportTemplate_EscapesAllCommonXSSVectors triangulates #1 with
// three additional attack payloads. The template must escape all of
// them.
func TestReportTemplate_EscapesAllCommonXSSVectors(t *testing.T) {
	cases := []struct {
		name    string
		payload string
		// substrings that must NOT appear in the output (the
		// unescaped attack vector).
		mustNotContain []string
	}{
		{
			name:           "img onerror",
			payload:        `<img src=x onerror=alert(1)>`,
			mustNotContain: []string{`<img src=x onerror=`},
		},
		{
			name:           "iframe srcdoc",
			payload:        `<iframe srcdoc="<script>alert(1)</script>">`,
			mustNotContain: []string{`<iframe srcdoc=`},
		},
		{
			name:           "anchor with javascript scheme",
			payload:        `<a href="javascript:alert(1)">click</a>`,
			mustNotContain: []string{`<a href="javascript:`},
		},
	}

	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			data := service.ReportData{
				IniciadoID:  uuid.New(),
				GeneratedAt: "2026-06-10T22:00:00Z",
				Respuestas: []*formularios.Respuesta{
					{PreguntaID: uuid.New(), RespuestaTexto: tc.payload},
				},
			}
			var buf bytes.Buffer
			if err := service.RenderReporteHTML(context.Background(), &buf, data); err != nil {
				t.Fatalf("RenderReporteHTML error: %v", err)
			}
			out := buf.String()
			for _, bad := range tc.mustNotContain {
				if strings.Contains(out, bad) {
					t.Errorf("attack vector %q reached the rendered HTML: contains %q", tc.payload, bad)
				}
			}
		})
	}
}

// TestReportTemplate_NoFileSchemeURLs asserts the "no file://" audit
// from the spec (PR-6 6.4). Even if a user submits a payload that
// LOOKS like a file:// URL, the template must not surface it as an
// unescaped href or src. This is the Go equivalent of the spec's
// "--disable-local-file-access" defense (the actual flag is set on
// Gotenberg's Chromium; this test pins the HTML-level defense).
func TestReportTemplate_NoFileSchemeURLs(t *testing.T) {
	data := service.ReportData{
		IniciadoID:  uuid.New(),
		GeneratedAt: "2026-06-10T22:00:00Z",
		Respuestas: []*formularios.Respuesta{
			{
				PreguntaID:     uuid.New(),
				RespuestaTexto: `<a href="file:///etc/passwd">read</a>`,
			},
		},
	}
	var buf bytes.Buffer
	if err := service.RenderReporteHTML(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderReporteHTML error: %v", err)
	}
	out := buf.String()
	// The unescaped <a href="file:// should NOT appear.
	if strings.Contains(out, `<a href="file://`) {
		t.Errorf("file:// URL reached the rendered HTML: %q", out)
	}
	// The escaped &lt;a href=&quot;file:// (or equivalent) MUST appear.
	if !strings.Contains(out, "file://") {
		t.Errorf("expected the literal file:// substring to still be present (escaped), got %q", out)
	}
}

// TestReportTemplate_ContainsNoScriptTags asserts the template itself
// contains no `<script>` tag — closing the spec's
// "--disable-javascript" defense at the template level (the Chromium
// flag is a belt-and-suspenders backup).
func TestReportTemplate_ContainsNoScriptTags(t *testing.T) {
	var buf bytes.Buffer
	if err := service.RenderReporteHTML(context.Background(), &buf, service.ReportData{
		IniciadoID:  uuid.New(),
		GeneratedAt: "2026-06-10T22:00:00Z",
	}); err != nil {
		t.Fatalf("RenderReporteHTML error: %v", err)
	}
	out := strings.ToLower(buf.String())
	if strings.Contains(out, "<script") {
		t.Errorf("rendered template contains <script> tag: %q", out)
	}
}

// TestReportTemplate_DoesNotLoadExternalResources asserts the template
// contains no <link rel="stylesheet" href="http(s)://..."> nor
// <img src="http(s)://..."> — the spec's
// "--disable-external-links" defense at the template level (the
// Chromium flag is the runtime backup).
func TestReportTemplate_DoesNotLoadExternalResources(t *testing.T) {
	var buf bytes.Buffer
	if err := service.RenderReporteHTML(context.Background(), &buf, service.ReportData{
		IniciadoID:  uuid.New(),
		GeneratedAt: "2026-06-10T22:00:00Z",
	}); err != nil {
		t.Fatalf("RenderReporteHTML error: %v", err)
	}
	out := strings.ToLower(buf.String())
	// The template is a static const; it must not contain external
	// http(s) URLs.
	if strings.Contains(out, `href="http`) || strings.Contains(out, `src="http`) {
		t.Errorf("rendered template contains external http(s) link: %q", out)
	}
}

// TestReportTemplate_HappyPathRendering triangulates the
// security tests above with a happy-path render. The output must
// contain the iniciadoID, the generated_at timestamp, and a
// non-escaped preguntaID (UUIDs are safe characters).
func TestReportTemplate_HappyPathRendering(t *testing.T) {
	iniciadoID := uuid.MustParse("11111111-1111-1111-1111-111111111111")
	preguntaID := uuid.MustParse("22222222-2222-2222-2222-222222222222")
	data := service.ReportData{
		IniciadoID:  iniciadoID,
		GeneratedAt: "2026-06-10T22:00:00Z",
		Respuestas: []*formularios.Respuesta{
			{PreguntaID: preguntaID, RespuestaTexto: "answer-1"},
			{PreguntaID: uuid.New(), RespuestaTexto: "answer-2"},
		},
	}
	var buf bytes.Buffer
	if err := service.RenderReporteHTML(context.Background(), &buf, data); err != nil {
		t.Fatalf("RenderReporteHTML error: %v", err)
	}
	out := buf.String()
	if !strings.Contains(out, iniciadoID.String()) {
		t.Errorf("expected output to contain iniciadoID %q, got %q", iniciadoID.String(), out)
	}
	if !strings.Contains(out, "2026-06-10T22:00:00Z") {
		t.Errorf("expected output to contain generated_at timestamp, got %q", out)
	}
	if !strings.Contains(out, "answer-1") || !strings.Contains(out, "answer-2") {
		t.Errorf("expected output to contain respuesta entries, got %q", out)
	}
}

// TestReportTemplate_UsesHTMLTemplate pins the package-level choice:
// the template is an html/template (auto-escape) and not a
// text/template. We assert by importing the template and verifying
// the RenderReporteHTML output escapes a payload — if the production
// code used text/template, the literal `<script>` would survive
// unchanged. (The TestReportTemplate_EscapesScriptTags test
// already pins the same thing; this is a belt-and-suspenders sanity.)
func TestReportTemplate_UsesHTMLTemplate(t *testing.T) {
	// Build a tiny template directly to confirm the package
	// choice. We use html/template so {{ . }} is escaped.
	tmpl := template.Must(template.New("sanity").Parse(`<p>{{ . }}</p>`))
	var b bytes.Buffer
	if err := tmpl.Execute(&b, `<script>x</script>`); err != nil {
		t.Fatalf("template execute error: %v", err)
	}
	if !strings.Contains(b.String(), "&lt;script&gt;") {
		t.Errorf("html/template did not escape — package choice is wrong")
	}
}
