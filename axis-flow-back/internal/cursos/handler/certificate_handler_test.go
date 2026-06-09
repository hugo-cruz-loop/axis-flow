package handler_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/handler"

	"github.com/go-chi/chi/v5"
	"github.com/google/uuid"
)

// ── mock ──────────────────────────────────────────────────────────────────────

type mockCertificateService struct {
	generateCertificateFn func(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error)
}

func (m *mockCertificateService) GenerateCertificate(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error) {
	return m.generateCertificateFn(ctx, empleadoID, examenID)
}

// ── tests ─────────────────────────────────────────────────────────────────────

func TestCertificateHandler_Download_Approved_200_ContentTypePDF(t *testing.T) {
	tenantID := uuid.New()
	pdfBytes := []byte("%PDF-1.4 fake-pdf-content")
	svc := &mockCertificateService{
		generateCertificateFn: func(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error) {
			return pdfBytes, "certificado_1_99.pdf", nil
		},
	}
	h := handler.NewCertificateHandler(svc)
	router := chi.NewRouter()
	router.Get("/certificado/{examen_id}/download", h.DownloadCertificate)

	req := httptest.NewRequest(http.MethodGet, "/certificado/1/download?empleado_id=99", nil)
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusOK {
		t.Fatalf("expected 200 got %d: %s", w.Code, w.Body.String())
	}
	ct := w.Header().Get("Content-Type")
	if ct != "application/pdf" {
		t.Fatalf("expected Content-Type application/pdf got %q", ct)
	}
	cd := w.Header().Get("Content-Disposition")
	if !strings.Contains(cd, "attachment") {
		t.Fatalf("expected Content-Disposition to contain 'attachment', got %q", cd)
	}
	if w.Body.Len() == 0 {
		t.Fatal("expected non-empty PDF body")
	}
}

func TestCertificateHandler_Download_NotApproved_403(t *testing.T) {
	tenantID := uuid.New()
	svc := &mockCertificateService{
		generateCertificateFn: func(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error) {
			return nil, "", cursos.ErrExamNotApproved
		},
	}
	h := handler.NewCertificateHandler(svc)
	router := chi.NewRouter()
	router.Get("/certificado/{examen_id}/download", h.DownloadCertificate)

	req := httptest.NewRequest(http.MethodGet, "/certificado/1/download?empleado_id=99", nil)
	req = withTenantCtx(req, tenantID)
	w := httptest.NewRecorder()

	router.ServeHTTP(w, req)

	if w.Code != http.StatusForbidden {
		t.Fatalf("expected 403 got %d: %s", w.Code, w.Body.String())
	}
}
