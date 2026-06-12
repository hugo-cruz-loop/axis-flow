package generator_test

import (
	"context"
	"errors"
	"testing"

	report "axis-flow-back/internal/report"
	"axis-flow-back/internal/report/generator"
)

func validPDFRequest() report.ReportRequest {
	clientID := 42
	return report.ReportRequest{
		Type:         "sales",
		OutputFormat: report.FormatPDF,
		UserID:       "user-1",
		EmpresaID:    "empresa-1",
		Filters: report.ReportFilters{
			DateFrom: "2024-01-01",
			DateTo:   "2024-12-31",
			ClientID: &clientID,
		},
	}
}

func TestPDFGenerator_Generate_ReturnsPDF(t *testing.T) {
	gen := generator.NewPDFGenerator()
	ctx := context.Background()

	data, err := gen.Generate(ctx, validPDFRequest())
	if err != nil {
		t.Fatalf("expected no error, got: %v", err)
	}
	if len(data) == 0 {
		t.Fatal("expected non-empty bytes")
	}
	if len(data) < 4 || string(data[:4]) != "%PDF" {
		t.Fatalf("expected PDF magic bytes %%PDF, got: %q", string(data[:min(4, len(data))]))
	}
}

func TestPDFGenerator_WrongFormat(t *testing.T) {
	gen := generator.NewPDFGenerator()
	ctx := context.Background()

	req := validPDFRequest()
	req.OutputFormat = report.FormatXLSX

	_, err := gen.Generate(ctx, req)
	if !errors.Is(err, report.ErrInvalidFormat) {
		t.Fatalf("expected ErrInvalidFormat, got: %v", err)
	}
}

func min(a, b int) int {
	if a < b {
		return a
	}
	return b
}
