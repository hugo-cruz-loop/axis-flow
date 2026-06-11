// Package service_test — PR-6 (6.1) TDD RED tests for the
// GotenbergPDFRenderer transport. The renderer is a thin wrapper
// around the existing internal/pdf/gotenberg.GotenbergClient
// (refactored from internal/cursos/gotenberg in PR-6 step 6.0).
//
// The tests assert the spec's "no PII / no vendor detail" surfacing
// (errors are wrapped with a generic message), and the seam between
// the service's safe HTML and Gotenberg's HTTP transport.
package service_test

import (
	"context"
	"errors"
	"io"
	"mime"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	formulariosService "axis-flow-back/internal/formularios/service"
	"axis-flow-back/internal/pdf/gotenberg"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// minimalValidPDFBytes is reused as the fake Gotenberg response.
var minimalValidPDFBytesForRenderer = []byte(
	"%PDF-1.4\n" +
		"1 0 obj<</Type/Catalog>>endobj\n" +
		"trailer<</Root 1 0 R>>\n" +
		"%%EOF\n",
)

// ---------------------------------------------------------------------------
// 6.1 (b) — GotenbergPDFRenderer tests
// ---------------------------------------------------------------------------

// TestGotenbergPDFRenderer_SendsHTMLEscapedToGotenberg asserts the
// end-to-end contract: the renderer submits the SAFE HTML (already
// auto-escaped by the service) to the Gotenberg endpoint, and the
// bytes Gotenberg returns flow through unchanged. The test stands
// up an httptest server simulating Gotenberg and inspects the
// multipart body.
func TestGotenbergPDFRenderer_SendsHTMLEscapedToGotenberg(t *testing.T) {
	var receivedBody []byte
	var receivedContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(minimalValidPDFBytesForRenderer)
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)

	html := "<html><body>safe &amp; escaped</body></html>"
	pdf, err := r.Render(context.Background(), html)
	require.NoError(t, err)
	assert.Equal(t, minimalValidPDFBytesForRenderer, pdf, "renderer must return Gotenberg's bytes unchanged")
	assert.True(t, strings.HasPrefix(receivedContentType, "multipart/form-data"),
		"expected multipart/form-data, got %q", receivedContentType)
	// The submitted multipart body MUST contain the HTML we sent.
	assert.Contains(t, string(receivedBody), "safe &amp; escaped",
		"Gotenberg received a different body than what we sent: %q", string(receivedBody))
}

// TestGotenbergPDFRenderer_RejectsScriptTagInSubmittedBody is the
// "no <script> reaches Gotenberg" audit (spec §Hardening, item 1
// "Strict HTML Auto-Escaping"). Even if a caller forgets to
// pre-escape (the service is responsible for that, not the
// renderer), the test pins the contract that whatever the renderer
// submits to Gotenberg is what the caller passed. The actual
// pre-escape guarantee lives in pdf_template.go's
// html/template; this test triangulates it.
func TestGotenbergPDFRenderer_RejectsScriptTagInSubmittedBody(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(minimalValidPDFBytesForRenderer)
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)

	// The caller is responsible for pre-escaping; we assert that
	// if a caller pre-escaped <script> to &lt;script&gt;, the
	// renderer submits the ESCAPED payload to Gotenberg. (A test
	// for the caller's pre-escape contract lives in
	// pdf_template_test.go.)
	const safeHTML = `<p>&lt;script&gt;alert(1)&lt;/script&gt;</p>`
	_, err := r.Render(context.Background(), safeHTML)
	require.NoError(t, err)
	assert.Contains(t, string(receivedBody), "&lt;script&gt;",
		"renderer must submit the escaped payload verbatim, got %q", string(receivedBody))
	assert.NotContains(t, string(receivedBody), "<script>alert",
		"renderer must NEVER submit an unescaped <script> tag")
}

// TestGotenbergPDFRenderer_RejectsFileSchemeInSubmittedBody is the
// "no file:// URLs reach PDF" audit. Same pattern as the
// script-tag test: we assert the renderer submits the caller's
// payload verbatim, and pin the caller's responsibility for
// pre-escaping file:// URLs to safe form.
func TestGotenbergPDFRenderer_RejectsFileSchemeInSubmittedBody(t *testing.T) {
	var receivedBody []byte
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		body, _ := io.ReadAll(r.Body)
		receivedBody = body
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(minimalValidPDFBytesForRenderer)
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)

	// Pre-escaped file:// URL (the html/template escape neutralizes
	// the file:// scheme in href / src contexts; a literal string
	// containing "file://" is fine to submit as text content).
	const safeHTML = `<p>&lt;a href=&quot;file:///etc/passwd&quot;&gt;read&lt;/a&gt;</p>`
	_, err := r.Render(context.Background(), safeHTML)
	require.NoError(t, err)
	assert.Contains(t, string(receivedBody), "&lt;a href=&quot;file:///etc/passwd&quot;&gt;",
		"renderer must submit the escaped payload verbatim, got %q", string(receivedBody))
	assert.NotContains(t, string(receivedBody), `<a href="file://`,
		"renderer must NEVER submit an unescaped file:// href")
}

// TestGotenbergPDFRenderer_5xxIsWrappedAsInternalError asserts the
// "no PII / no vendor detail" surfacing convention: a Gotenberg
// 500 must be reported to the caller as a generic formularios
// internal error, with the raw status code suppressed. The
// underlying error is logged via the slog layer in the PDF service
// (not in the renderer — the renderer stays a thin transport).
func TestGotenbergPDFRenderer_5xxIsWrappedAsInternalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusInternalServerError)
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)

	_, err := r.Render(context.Background(), "<html></html>")
	require.Error(t, err)
	// No PII / vendor detail in the surfaced error.
	assert.NotContains(t, err.Error(), "500", "surfaces no HTTP status code to caller")
	assert.NotContains(t, err.Error(), "Internal Server Error", "surfaces no HTTP reason phrase")
	assert.NotContains(t, err.Error(), srv.URL, "surfaces no internal endpoint")
	// The error class is the generic one.
	assert.True(t,
		errors.Is(err, formulariosService.ErrPDFRenderInternal) || strings.HasPrefix(err.Error(), "pdf: render:"),
		"error must be the generic internal class, got %v", err)
}

// TestGotenbergPDFRenderer_4xxIsWrappedAsInternalError triangulates
// the 5xx test: a 4xx from Gotenberg (e.g. the chromium engine
// crashing on the HTML) is also surfaced as the generic internal
// error.
func TestGotenbergPDFRenderer_4xxIsWrappedAsInternalError(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusBadRequest)
		_, _ = w.Write([]byte("chromium: page crash"))
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)

	_, err := r.Render(context.Background(), "<html></html>")
	require.Error(t, err)
	assert.NotContains(t, err.Error(), "400", "surfaces no HTTP status code to caller")
	assert.NotContains(t, err.Error(), "chromium", "surfaces no vendor detail")
}

// TestGotenbergPDFRenderer_ContextCancellationPropagates asserts
// that the renderer's HTTP request honors the caller's context.
// This is critical for the render-pool bounded-wait pattern (PR-6
// 6.3) — when a context is cancelled, the in-flight Gotenberg
// request must abort and the error must surface.
func TestGotenbergPDFRenderer_ContextCancellationPropagates(t *testing.T) {
	gotenbergResponded := make(chan struct{})
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		<-gotenbergResponded // block until the test signals
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(minimalValidPDFBytesForRenderer)
	}))
	defer srv.Close()
	defer close(gotenbergResponded)

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)

	ctx, cancel := context.WithTimeout(context.Background(), 50*time.Millisecond)
	defer cancel()

	_, err := r.Render(ctx, "<html></html>")
	require.Error(t, err, "context cancellation must abort the request")
	// Gotenberg client wraps with "gotenberg: request failed" — we
	// don't pin that exact string (PII-style), only that an error
	// is returned.
	assert.True(t, err != nil, "error returned on context cancellation")
}

// TestGotenbergPDFRenderer_MockClientPath asserts the renderer also
// works with the mock client (dev/test environment without a real
// Gotenberg instance). This pins the dependency-injection seam:
// the renderer is constructor-injected with a GotenbergClient
// (interface) — both real and mock clients satisfy it.
func TestGotenbergPDFRenderer_MockClientPath(t *testing.T) {
	client := gotenberg.New("", false) // mock
	r := formulariosService.NewGotenbergPDFRenderer(client)

	pdf, err := r.Render(context.Background(), "<html></html>")
	require.NoError(t, err)
	assert.NotEmpty(t, pdf, "mock client must return non-empty bytes")
}

// TestGotenbergPDFRenderer_SentContentTypeIsMultipart pins the
// multipart/form-data contract. Gotenberg expects the HTML as a
// "files" form file; any other content-type is rejected.
func TestGotenbergPDFRenderer_SentContentTypeIsMultipart(t *testing.T) {
	var receivedContentType string
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		receivedContentType = r.Header.Get("Content-Type")
		w.WriteHeader(http.StatusOK)
		_, _ = w.Write(minimalValidPDFBytesForRenderer)
	}))
	defer srv.Close()

	client := gotenberg.New(srv.URL, true)
	r := formulariosService.NewGotenbergPDFRenderer(client)
	_, err := r.Render(context.Background(), "<html></html>")
	require.NoError(t, err)

	mediatype, params, err := mime.ParseMediaType(receivedContentType)
	require.NoError(t, err)
	assert.Equal(t, "multipart/form-data", mediatype, "content-type top-level must be multipart/form-data")
	assert.NotEmpty(t, params["boundary"], "multipart content-type must carry a boundary param")
	_ = multipart.NewWriter // keep import alive for the linter
}
