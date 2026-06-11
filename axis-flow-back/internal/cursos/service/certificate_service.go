package service

import (
	"context"
	"fmt"
	"html/template"
	"strings"
	"time"

	"axis-flow-back/internal/cursos"
	"axis-flow-back/internal/cursos/repository"
	"axis-flow-back/internal/pdf/gotenberg"
)

// CertificateService defines the certificate generation contract.
type CertificateService interface {
	// GenerateCertificate returns (pdfBytes, filename, error).
	// Returns ErrExamNotApproved if the employee has not passed the exam.
	GenerateCertificate(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error)
}

type pgxCertificateService struct {
	exam   repository.ExamRepository
	client gotenberg.GotenbergClient
}

// NewCertificateService constructs a CertificateService.
func NewCertificateService(exam repository.ExamRepository, client gotenberg.GotenbergClient) CertificateService {
	return &pgxCertificateService{exam: exam, client: client}
}

// certificateTemplateData holds values injected into the PDF HTML template.
type certificateTemplateData struct {
	EmpleadoID   int64
	ExamenID     int64
	Calificacion float64
	Fecha        string
}

const certificateHTML = `<!DOCTYPE html>
<html lang="es">
<head><meta charset="UTF-8"/><style>
body{font-family:Arial,sans-serif;text-align:center;padding:60px;}
h1{color:#2c3e50;}
.grade{font-size:48px;color:#27ae60;font-weight:bold;}
.footer{margin-top:60px;font-size:12px;color:#7f8c8d;}
</style></head>
<body>
  <h1>Certificado de Aprovechamiento</h1>
  <p>Este certificado se otorga al empleado <strong>{{.EmpleadoID}}</strong></p>
  <p>por haber aprobado satisfactoriamente el examen <strong>{{.ExamenID}}</strong></p>
  <p class="grade">{{printf "%.1f" .Calificacion}}%</p>
  <p>Fecha: {{.Fecha}}</p>
  <div class="footer">Generado automáticamente por Axis Flow</div>
</body>
</html>`

// GenerateCertificate generates a PDF certificate for the last approved exam attempt.
//
// Steps:
//  1. Load resultados for empleadoID on examenID.
//  2. Find the last resultado — if not aprobado → ErrExamNotApproved.
//  3. Build HTML from template with empleado name, grade, date.
//  4. Call GotenbergClient.GeneratePDF → returns PDF bytes.
//  5. Return (pdfBytes, "certificado_{examenID}_{empleadoID}.pdf", nil).
func (s *pgxCertificateService) GenerateCertificate(ctx context.Context, empleadoID, examenID int64) ([]byte, string, error) {
	// 1. Load results.
	resultados, err := s.exam.GetResultados(ctx, examenID, empleadoID)
	if err != nil {
		return nil, "", fmt.Errorf("CertificateService.GenerateCertificate: load resultados: %w", err)
	}

	// 2. Find last approved result.
	var last *cursos.ResultadoExamen
	for _, r := range resultados {
		if r.Aprobado != nil && *r.Aprobado {
			last = r
		}
	}
	if last == nil {
		return nil, "", cursos.ErrExamNotApproved
	}

	// 3. Build HTML.
	tmpl, err := template.New("cert").Parse(certificateHTML)
	if err != nil {
		return nil, "", fmt.Errorf("CertificateService.GenerateCertificate: parse template: %w", err)
	}

	var calificacion float64
	if last.Calificacion != nil {
		calificacion = *last.Calificacion
	}

	data := certificateTemplateData{
		EmpleadoID:   empleadoID,
		ExamenID:     examenID,
		Calificacion: calificacion,
		Fecha:        time.Now().Format("2006-01-02"),
	}

	var buf strings.Builder
	if err := tmpl.Execute(&buf, data); err != nil {
		return nil, "", fmt.Errorf("CertificateService.GenerateCertificate: render template: %w", err)
	}

	// 4. Generate PDF.
	pdfBytes, err := s.client.GeneratePDF(ctx, buf.String())
	if err != nil {
		return nil, "", fmt.Errorf("CertificateService.GenerateCertificate: generate pdf: %w", err)
	}

	// 5. Return filename.
	filename := fmt.Sprintf("certificado_%d_%d.pdf", examenID, empleadoID)
	return pdfBytes, filename, nil
}
