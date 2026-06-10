// Package handler_test — respuesta handler tests (PR-4 task 4.3).
//
// Mirrors internal/atencionseguimiento/handler/queja_handler_test.go
// (PR-4 of 09_AtencionSeguimiento_Service_Spec). Adds multipart-form
// content-negotiation tests for the respuesta endpoint — the
// formularios openapi spec defines POST /respuesta as accepting BOTH
// application/json AND multipart/form-data, so the handler must
// auto-detect and route accordingly.
package handler_test

import (
	"bytes"
	"context"
	"encoding/json"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"axis-flow-back/internal/formularios"
	formshandler "axis-flow-back/internal/formularios/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// ---------------------------------------------------------------------------
// mockRespuestaService — implements service.RespuestaService.
// ---------------------------------------------------------------------------

type mockRespuestaService struct {
	submitRespuestaFn        func(ctx context.Context, r *formularios.Respuesta, empresaID uuid.UUID) (*formularios.Respuesta, error)
	getRespuestasByIniciadoFn func(ctx context.Context, iniciadoID, empresaID uuid.UUID) ([]*formularios.Respuesta, error)
}

func (m *mockRespuestaService) SubmitRespuesta(ctx context.Context, r *formularios.Respuesta, empresaID uuid.UUID) (*formularios.Respuesta, error) {
	return m.submitRespuestaFn(ctx, r, empresaID)
}
func (m *mockRespuestaService) GetRespuestasByIniciado(ctx context.Context, iniciadoID, empresaID uuid.UUID) ([]*formularios.Respuesta, error) {
	if m.getRespuestasByIniciadoFn == nil {
		return nil, nil
	}
	return m.getRespuestasByIniciadoFn(ctx, iniciadoID, empresaID)
}

// ---------------------------------------------------------------------------
// mockPDFService — implements service.PDFService.
// ---------------------------------------------------------------------------

type mockPDFService struct {
	generateReporteFn func(ctx context.Context, iniciadoID, empresaID uuid.UUID) (string, error)
}

func (m *mockPDFService) GenerateReporte(ctx context.Context, iniciadoID, empresaID uuid.UUID) (string, error) {
	return m.generateReporteFn(ctx, iniciadoID, empresaID)
}

// respuestaRouter mounts the two respuesta routes on a chi router.
func respuestaRouter(h *formshandler.RespuestaHandler) *chi.Mux {
	r := chi.NewRouter()
	r.Post("/respuesta", h.SubmitRespuesta)
	r.Get("/respuesta/reporte/pregunta/{id}/pdf", h.GetReportePDF)
	return r
}

// ---------------------------------------------------------------------------
// POST /respuesta — JSON path.
// ---------------------------------------------------------------------------

func TestSubmitRespuesta_JSON_NoJWT_Returns401(t *testing.T) {
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	body, _ := json.Marshal(map[string]any{
		"evento_iniciado_id": uuid.New(),
		"formulario_id":      uuid.New(),
		"pregunta_id":        uuid.New(),
		"respuesta_lista":    map[string]any{},
	})
	r := httptest.NewRequest(http.MethodPost, "/respuesta", bytes.NewReader(body))
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestSubmitRespuesta_JSON_MissingIniciadoID_Returns422(t *testing.T) {
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"formulario_id":   uuid.New(),
		"pregunta_id":     uuid.New(),
		"respuesta_lista": map[string]any{},
		// evento_iniciado_id is missing
	})
	r := httptest.NewRequest(http.MethodPost, "/respuesta", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestSubmitRespuesta_JSON_MissingRespuestaLista_Returns422(t *testing.T) {
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	tenantID := uuid.New()
	userID := uuid.New()
	body, _ := json.Marshal(map[string]any{
		"evento_iniciado_id": uuid.New(),
		"formulario_id":      uuid.New(),
		"pregunta_id":        uuid.New(),
		// respuesta_lista is missing
	})
	r := httptest.NewRequest(http.MethodPost, "/respuesta", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "respuesta_lista is required per openapi")
}

func TestSubmitRespuesta_JSON_ValidRequest_Returns201WithData(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	iniciadoID := uuid.New()
	formID := uuid.New()
	pregID := uuid.New()
	created := &formularios.Respuesta{
		ID:               uuid.New(),
		EventoIniciadoID: iniciadoID,
		PreguntaID:       pregID,
		RespuestaTexto:   "ok",
	}
	svc := &mockRespuestaService{
		submitRespuestaFn: func(_ context.Context, r *formularios.Respuesta, _ uuid.UUID) (*formularios.Respuesta, error) {
			require.Equal(t, iniciadoID, r.EventoIniciadoID)
			require.Equal(t, pregID, r.PreguntaID)
			require.Equal(t, "ok", r.RespuestaTexto)
			return created, nil
		},
	}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	body, _ := json.Marshal(map[string]any{
		"evento_iniciado_id": iniciadoID,
		"formulario_id":      formID,
		"pregunta_id":        pregID,
		"respuesta_texto":    "ok",
		"respuesta_lista":    map[string]any{"seleccion": []any{}},
		"evidencia_urls":     []string{"https://s3.example.com/evidence1.jpg"},
	})
	r := httptest.NewRequest(http.MethodPost, "/respuesta", bytes.NewReader(body))
	r.Header.Set("Content-Type", "application/json")
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy JSON path → 201, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	data, ok := env["data"].(map[string]any)
	require.True(t, ok, "expected data object, got %T", env["data"])
	assert.Equal(t, created.ID.String(), data["id"])
	assert.Equal(t, iniciadoID.String(), data["evento_iniciado_id"])
	assert.Equal(t, pregID.String(), data["pregunta_id"])
}

// ---------------------------------------------------------------------------
// POST /respuesta — Multipart path.
// ---------------------------------------------------------------------------

// buildMultipartRespuesta creates a multipart/form-data body with the
// minimum required fields and three evidence files.
func buildMultipartRespuesta(t *testing.T) (*bytes.Buffer, string) {
	t.Helper()
	var b bytes.Buffer
	w := multipart.NewWriter(&b)
	require.NoError(t, w.WriteField("evento_iniciado_id", uuid.New().String()))
	require.NoError(t, w.WriteField("formulario_id", uuid.New().String()))
	require.NoError(t, w.WriteField("pregunta_id", uuid.New().String()))
	require.NoError(t, w.WriteField("respuesta_texto", "ok"))
	require.NoError(t, w.WriteField("respuesta_lista", `{"seleccion":[]}`))
	// Three evidence files of the whitelisted types.
	for i, ct := range []string{"image/jpeg", "image/png", "application/pdf"} {
		fw, err := w.CreateFormFile("evidencia"+itoa(i+1), "ev"+itoa(i+1)+extFor(ct))
		require.NoError(t, err)
		_, err = fw.Write([]byte("fake-bytes-" + ct))
		require.NoError(t, err)
	}
	require.NoError(t, w.Close())
	return &b, w.FormDataContentType()
}

func itoa(i int) string {
	if i == 0 {
		return "0"
	}
	neg := false
	if i < 0 {
		neg = true
		i = -i
	}
	var b [20]byte
	bp := len(b)
	for i > 0 {
		bp--
		b[bp] = byte('0' + i%10)
		i /= 10
	}
	if neg {
		bp--
		b[bp] = '-'
	}
	return string(b[bp:])
}

func extFor(ct string) string {
	switch ct {
	case "image/jpeg":
		return ".jpg"
	case "image/png":
		return ".png"
	case "image/webp":
		return ".webp"
	case "application/pdf":
		return ".pdf"
	}
	return ".bin"
}

func TestSubmitRespuesta_Multipart_ValidRequest_Returns201WithData(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	created := &formularios.Respuesta{ID: uuid.New()}
	svc := &mockRespuestaService{
		submitRespuestaFn: func(_ context.Context, r *formularios.Respuesta, _ uuid.UUID) (*formularios.Respuesta, error) {
			require.NotEqual(t, uuid.Nil, r.EventoIniciadoID, "iniciadoID must be parsed from form field")
			require.NotEqual(t, uuid.Nil, r.PreguntaID, "preguntaID must be parsed from form field")
			require.Equal(t, "ok", r.RespuestaTexto)
			// Multipart: evidencia1/2/3 URLs are NOT stored directly;
			// PR-6 will wire S3 upload. For PR-4 the handler records
			// only that files were present (synthetic marker).
			assert.NotEmpty(t, r.Evidencia1, "evidencia1 marker must be set")
			assert.NotEmpty(t, r.Evidencia2)
			assert.NotEmpty(t, r.Evidencia3)
			return created, nil
		},
	}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	body, ct := buildMultipartRespuesta(t)
	r := httptest.NewRequest(http.MethodPost, "/respuesta", body)
	r.Header.Set("Content-Type", ct)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	require.Equal(t, http.StatusCreated, w.Code, "happy multipart path → 201, body: %s", w.Body.String())
	var env map[string]any
	require.NoError(t, json.Unmarshal(w.Body.Bytes(), &env))
	assert.Equal(t, true, env["success"])
	data, ok := env["data"].(map[string]any)
	require.True(t, ok)
	assert.Equal(t, created.ID.String(), data["id"])
}

func TestSubmitRespuesta_Multipart_DisallowedContentType_Returns422(t *testing.T) {
	svc := &mockRespuestaService{
		submitRespuestaFn: func(_ context.Context, _ *formularios.Respuesta, _ uuid.UUID) (*formularios.Respuesta, error) {
			t.Fatal("service must not be called when content-type is not whitelisted")
			return nil, nil
		},
	}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("evento_iniciado_id", uuid.New().String())
	_ = mw.WriteField("formulario_id", uuid.New().String())
	_ = mw.WriteField("pregunta_id", uuid.New().String())
	_ = mw.WriteField("respuesta_lista", `{"seleccion":[]}`)
	fw, _ := mw.CreateFormFile("evidencia1", "evil.exe")
	_, _ = fw.Write([]byte("fake-bytes"))
	_ = mw.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodPost, "/respuesta", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code, "disallowed content-type → 422")
	assert.Contains(t, w.Body.String(), "disallowed", "error message should mention disallowed content-type")
}

func TestSubmitRespuesta_Multipart_TooManyFiles_Returns422(t *testing.T) {
	svc := &mockRespuestaService{
		submitRespuestaFn: func(_ context.Context, _ *formularios.Respuesta, _ uuid.UUID) (*formularios.Respuesta, error) {
			t.Fatal("service must not be called when more than 3 files are present")
			return nil, nil
		},
	}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	var b bytes.Buffer
	mw := multipart.NewWriter(&b)
	_ = mw.WriteField("evento_iniciado_id", uuid.New().String())
	_ = mw.WriteField("formulario_id", uuid.New().String())
	_ = mw.WriteField("pregunta_id", uuid.New().String())
	_ = mw.WriteField("respuesta_lista", `{"seleccion":[]}`)
	// 2 files in evidencia1 — exceeds the per-key cap of 1 and
	// therefore the total cap of 3.
	for i := 1; i <= 2; i++ {
		fw, _ := mw.CreateFormFile("evidencia1", "ev"+itoa(i)+".jpg")
		_, _ = fw.Write([]byte("fake"))
	}
	_ = mw.Close()

	tenantID := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodPost, "/respuesta", &b)
	r.Header.Set("Content-Type", mw.FormDataContentType())
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
	assert.Contains(t, w.Body.String(), "too many", "error must mention file count limit")
}

func TestSubmitRespuesta_UnsupportedContentType_Returns422(t *testing.T) {
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)

	tenantID := uuid.New()
	userID := uuid.New()
	body := strings.NewReader("not json, not multipart")
	r := httptest.NewRequest(http.MethodPost, "/respuesta", body)
	r.Header.Set("Content-Type", "text/plain")
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	h.SubmitRespuesta(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

// ---------------------------------------------------------------------------
// GET /respuesta/reporte/pregunta/{id}/pdf — PDF stream.
// ---------------------------------------------------------------------------

func TestGetReportePDF_NoJWT_Returns401(t *testing.T) {
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)
	router := respuestaRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/respuesta/reporte/pregunta/"+uuid.New().String()+"/pdf", nil)
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnauthorized, w.Code)
}

func TestGetReportePDF_InvalidUUID_Returns422(t *testing.T) {
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{}
	h := formshandler.NewRespuestaHandler(svc, pdf)
	router := respuestaRouter(h)

	tenantID := uuid.New()
	userID := uuid.New()
	r := httptest.NewRequest(http.MethodGet, "/respuesta/reporte/pregunta/not-a-uuid/pdf", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusUnprocessableEntity, w.Code)
}

func TestGetReportePDF_ValidRequest_Returns200WithPDFBody(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	preguntaID := uuid.New()
	pdfURL := "https://s3.example.com/reports/test.pdf"
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{
		generateReporteFn: func(_ context.Context, _ uuid.UUID, _ uuid.UUID) (string, error) {
			return pdfURL, nil
		},
	}
	h := formshandler.NewRespuestaHandler(svc, pdf)
	router := respuestaRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/respuesta/reporte/pregunta/"+preguntaID.String()+"/pdf", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	require.Equal(t, http.StatusOK, w.Code, "happy path → 200, body: %s", w.Body.String())
	assert.Equal(t, "application/pdf", w.Header().Get("Content-Type"))
	assert.Contains(t, w.Header().Get("Content-Disposition"), "reporte-")
	assert.Equal(t, pdfURL, w.Header().Get("X-PDF-URL"))
}

func TestGetReportePDF_ServiceErrNotFound_Returns404(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	preguntaID := uuid.New()
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{
		generateReporteFn: func(_ context.Context, _, _ uuid.UUID) (string, error) {
			return "", formularios.ErrNotFound
		},
	}
	h := formshandler.NewRespuestaHandler(svc, pdf)
	router := respuestaRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/respuesta/reporte/pregunta/"+preguntaID.String()+"/pdf", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	assert.Equal(t, http.StatusNotFound, w.Code)
}

// No-PII regression: the PDF download path must NOT echo internal
// error text (S3 paths, render-error details) into the response body.
func TestGetReportePDF_GenericErrorMessageHasNoPII(t *testing.T) {
	tenantID := uuid.New()
	userID := uuid.New()
	preguntaID := uuid.New()
	svc := &mockRespuestaService{}
	pdf := &mockPDFService{
		generateReporteFn: func(_ context.Context, _, _ uuid.UUID) (string, error) {
			return "", assertAnErrorWithPII("wkhtmltopdf: signal killed on /tmp/formularios/<secret>")
		},
	}
	h := formshandler.NewRespuestaHandler(svc, pdf)
	router := respuestaRouter(h)

	r := httptest.NewRequest(http.MethodGet, "/respuesta/reporte/pregunta/"+preguntaID.String()+"/pdf", nil)
	r = injectFormulariosCtx(r, tenantID.String(), userID.String(), 1, "Empleado")
	w := httptest.NewRecorder()
	router.ServeHTTP(w, r)

	body := w.Body.String()
	assert.Equal(t, http.StatusInternalServerError, w.Code)
	assert.NotContains(t, body, "wkhtmltopdf", "vendor detail must not leak")
	assert.NotContains(t, body, "/tmp/formularios", "internal paths must not leak")
}

// assertAnErrorWithPII is a tiny helper that returns an error whose
// message contains the leaky text; the test asserts the response body
// does NOT contain the substring.
type piiError struct{ msg string }

func (e *piiError) Error() string { return e.msg }
func assertAnErrorWithPII(s string) error {
	return &piiError{msg: s}
}
