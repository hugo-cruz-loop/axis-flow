package handler_test

import (
	"bytes"
	"context"
	"errors"
	"io"
	"mime/multipart"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"axis-flow-back/internal/bolsatrabajo"
	"axis-flow-back/internal/bolsatrabajo/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockPostulacionService struct {
	applyFn      func(ctx context.Context, p *bolsatrabajo.Postulacion, cvReader io.Reader, cvSize int64, token, ip string) (*bolsatrabajo.Postulacion, error)
	getStatsFn   func(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error)
	updateFn     func(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error)
}

func (m *mockPostulacionService) Apply(ctx context.Context, p *bolsatrabajo.Postulacion, cvReader io.Reader, cvSize int64, token, ip string) (*bolsatrabajo.Postulacion, error) {
	return m.applyFn(ctx, p, cvReader, cvSize, token, ip)
}
func (m *mockPostulacionService) GetStatsByTrabajo(ctx context.Context, trabajoID, empresaID uuid.UUID) (*bolsatrabajo.PipelineStats, error) {
	return m.getStatsFn(ctx, trabajoID, empresaID)
}
func (m *mockPostulacionService) UpdateEstatus(ctx context.Context, id, empresaID uuid.UUID, newEstatus int) (*bolsatrabajo.Postulacion, error) {
	return m.updateFn(ctx, id, empresaID, newEstatus)
}

// ── helpers ───────────────────────────────────────────────────────────────────

func buildPostulacionRouter(svc handler.PostulacionServicer) http.Handler {
	r := chi.NewRouter()
	h := handler.NewPostulacionHandler(svc)
	r.Post("/bolsa-trabajo/postulacion/apply", h.Apply)
	r.Get("/bolsa-trabajo/postulacion/by-trabajo-stats/{id}", h.GetStats)
	r.Patch("/bolsa-trabajo/postulacion/status/{id}", h.UpdateEstatus)
	return r
}

// buildMultipartCV creates a multipart/form-data body with a CV file field.
func buildMultipartCV(t *testing.T, fieldName, filename string, content []byte, extraFields map[string]string) (*bytes.Buffer, string) {
	t.Helper()
	var buf bytes.Buffer
	mw := multipart.NewWriter(&buf)
	for k, v := range extraFields {
		_ = mw.WriteField(k, v)
	}
	fw, err := mw.CreateFormFile(fieldName, filename)
	if err != nil {
		t.Fatal(err)
	}
	_, _ = fw.Write(content)
	mw.Close()
	return &buf, mw.FormDataContentType()
}

// pdfMagic returns a minimal PDF magic-byte prefix (enough to pass magic validation).
func pdfMagic() []byte {
	return append([]byte("%PDF-1.4\n"), bytes.Repeat([]byte("x"), 100)...)
}

// ── tests ─────────────────────────────────────────────────────────────────────

// PDF valid → 201
func TestApply_PDFValid(t *testing.T) {
	result := &bolsatrabajo.Postulacion{
		ID:             uuid.New(),
		TrabajoID:      uuid.New(),
		NombreCompleto: "John Doe",
		Email:          "john@example.com",
		Estatus:        bolsatrabajo.PostulacionPendiente,
		CreatedAt:      time.Now(),
	}
	svc := &mockPostulacionService{
		applyFn: func(_ context.Context, _ *bolsatrabajo.Postulacion, _ io.Reader, _ int64, _, _ string) (*bolsatrabajo.Postulacion, error) {
			return result, nil
		},
	}
	router := buildPostulacionRouter(svc)

	fields := map[string]string{
		"trabajoId":      uuid.New().String(),
		"nombreCompleto": "John Doe",
		"email":          "john@example.com",
		"telefono":       "5551234567",
		"turnstileToken": "valid-token",
	}
	body, ct := buildMultipartCV(t, "cv", "resume.pdf", pdfMagic(), fields)
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/postulacion/apply", body)
	req.Header.Set("Content-Type", ct)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusCreated {
		t.Fatalf("expected 201, got %d — body: %s", w.Code, w.Body.String())
	}
}

// file > 5MB → 422
func TestApply_FileTooLarge(t *testing.T) {
	svc := &mockPostulacionService{
		applyFn: func(_ context.Context, _ *bolsatrabajo.Postulacion, _ io.Reader, _ int64, _, _ string) (*bolsatrabajo.Postulacion, error) {
			return nil, bolsatrabajo.ErrInvalidFile
		},
	}
	router := buildPostulacionRouter(svc)

	// Generate 6MB content
	bigContent := bytes.Repeat(pdfMagic(), 6*1024*1024/len(pdfMagic())+1)
	fields := map[string]string{
		"trabajoId":      uuid.New().String(),
		"nombreCompleto": "Jane",
		"email":          "jane@example.com",
		"telefono":       "5550000000",
		"turnstileToken": "tok",
	}
	body, ct := buildMultipartCV(t, "cv", "big.pdf", bigContent, fields)
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/postulacion/apply", body)
	req.Header.Set("Content-Type", ct)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusUnprocessableEntity {
		t.Fatalf("expected 422, got %d — body: %s", w.Code, w.Body.String())
	}
}

// rate limit exceeded → 429
func TestApply_RateLimit(t *testing.T) {
	svc := &mockPostulacionService{
		applyFn: func(_ context.Context, _ *bolsatrabajo.Postulacion, _ io.Reader, _ int64, _, _ string) (*bolsatrabajo.Postulacion, error) {
			return &bolsatrabajo.Postulacion{ID: uuid.New()}, nil
		},
	}

	// Build a handler with very tight rate limit (1 req / large window so second always fails)
	h := handler.NewPostulacionHandler(svc, handler.WithRateLimit(1, time.Hour))
	r := chi.NewRouter()
	r.Post("/bolsa-trabajo/postulacion/apply", h.Apply)
	router := r

	fields := map[string]string{
		"trabajoId":      uuid.New().String(),
		"nombreCompleto": "Bob",
		"email":          "bob@example.com",
		"telefono":       "5559999999",
		"turnstileToken": "tok",
	}

	// First request — should pass
	body1, ct1 := buildMultipartCV(t, "cv", "cv.pdf", pdfMagic(), fields)
	req1 := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/postulacion/apply", body1)
	req1.Header.Set("Content-Type", ct1)
	req1.RemoteAddr = "1.2.3.4:1234"
	w1 := httptest.NewRecorder()
	router.ServeHTTP(w1, req1)
	if w1.Code != http.StatusCreated {
		t.Fatalf("first request expected 201, got %d", w1.Code)
	}

	// Second request from same IP — rate limited
	body2, ct2 := buildMultipartCV(t, "cv", "cv.pdf", pdfMagic(), fields)
	req2 := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/postulacion/apply", body2)
	req2.Header.Set("Content-Type", ct2)
	req2.RemoteAddr = "1.2.3.4:5678"
	w2 := httptest.NewRecorder()
	router.ServeHTTP(w2, req2)
	if w2.Code != http.StatusTooManyRequests {
		t.Fatalf("second request expected 429, got %d", w2.Code)
	}
}

// Turnstile fail → 400 with CAPTCHA_VALIDATION_FAILED
func TestApply_TurnstileFail(t *testing.T) {
	svc := &mockPostulacionService{
		applyFn: func(_ context.Context, _ *bolsatrabajo.Postulacion, _ io.Reader, _ int64, _, _ string) (*bolsatrabajo.Postulacion, error) {
			return nil, bolsatrabajo.ErrCaptchaFail
		},
	}
	router := buildPostulacionRouter(svc)

	fields := map[string]string{
		"trabajoId":      uuid.New().String(),
		"nombreCompleto": "Alice",
		"email":          "alice@example.com",
		"telefono":       "5550001111",
		"turnstileToken": "bad-token",
	}
	body, ct := buildMultipartCV(t, "cv", "cv.pdf", pdfMagic(), fields)
	req := httptest.NewRequest(http.MethodPost, "/bolsa-trabajo/postulacion/apply", body)
	req.Header.Set("Content-Type", ct)

	w := httptest.NewRecorder()
	router.ServeHTTP(w, req)

	if w.Code != http.StatusBadRequest {
		t.Fatalf("expected 400, got %d — body: %s", w.Code, w.Body.String())
	}
	if !strings.Contains(w.Body.String(), "CAPTCHA_VALIDATION_FAILED") {
		t.Fatalf("expected CAPTCHA_VALIDATION_FAILED in body, got: %s", w.Body.String())
	}
}

// Verify errors import is used.
var _ = errors.New
