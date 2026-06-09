package service_test

import (
	"context"
	"errors"
	"testing"
	"time"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/service"
)

// --- mock GotenbergClient ---

type mockPDFClient struct {
	generatePDF func(ctx context.Context, htmlContent string) ([]byte, error)
}

func (m *mockPDFClient) GeneratePDF(ctx context.Context, htmlContent string) ([]byte, error) {
	if m.generatePDF != nil {
		return m.generatePDF(ctx, htmlContent)
	}
	return []byte("%PDF-1.4 mock"), nil
}

// reuse mockExamRepo from exam_service_test.go (same package)

func aprobado(v bool) *bool { return &v }
func grade(v float64) *float64 { return &v }

// --- Tests ---

func TestGenerateCertificate_NotApproved_Error(t *testing.T) {
	examRepo := &mockExamRepo{
		getResultados: func(_ context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
			return []*cursos.ResultadoExamen{
				{
					ID: 1, ExamenID: examenID, EmpleadoID: empleadoID,
					Aprobado: aprobado(false), Calificacion: grade(50),
					Intento: 1, CreatedAt: time.Now(),
				},
			}, nil
		},
	}

	pdfClient := &mockPDFClient{}
	svc := service.NewCertificateService(examRepo, pdfClient)

	_, _, err := svc.GenerateCertificate(context.Background(), 10, 1)
	if !errors.Is(err, cursos.ErrExamNotApproved) {
		t.Fatalf("expected ErrExamNotApproved, got %v", err)
	}
}

func TestGenerateCertificate_Approved_ReturnsPDFBytes(t *testing.T) {
	examRepo := &mockExamRepo{
		getResultados: func(_ context.Context, examenID, empleadoID int64) ([]*cursos.ResultadoExamen, error) {
			return []*cursos.ResultadoExamen{
				{
					ID: 1, ExamenID: examenID, EmpleadoID: empleadoID,
					Aprobado: aprobado(true), Calificacion: grade(85),
					Intento: 1, CreatedAt: time.Now(),
				},
			}, nil
		},
	}

	pdfClient := &mockPDFClient{
		generatePDF: func(_ context.Context, _ string) ([]byte, error) {
			return []byte("%PDF-1.4 real-cert"), nil
		},
	}

	svc := service.NewCertificateService(examRepo, pdfClient)

	pdfBytes, filename, err := svc.GenerateCertificate(context.Background(), 10, 1)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if len(pdfBytes) == 0 {
		t.Fatal("expected non-empty PDF bytes")
	}
	expectedFilename := "certificado_1_10.pdf"
	if filename != expectedFilename {
		t.Fatalf("expected filename %q, got %q", expectedFilename, filename)
	}
}

func TestGenerateCertificate_NoResultados_Error(t *testing.T) {
	examRepo := &mockExamRepo{
		getResultados: func(_ context.Context, _, _ int64) ([]*cursos.ResultadoExamen, error) {
			return nil, nil // empty results
		},
	}

	svc := service.NewCertificateService(examRepo, &mockPDFClient{})
	_, _, err := svc.GenerateCertificate(context.Background(), 10, 1)
	if !errors.Is(err, cursos.ErrExamNotApproved) {
		t.Fatalf("expected ErrExamNotApproved when no results, got %v", err)
	}
}
