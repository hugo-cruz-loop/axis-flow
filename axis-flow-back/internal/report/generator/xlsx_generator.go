package generator

import (
	"bytes"
	"context"
	"fmt"

	"github.com/xuri/excelize/v2"

	report "axis-flow-back/internal/report"
)

// XLSXGenerator generates report files in XLSX format.
type XLSXGenerator interface {
	Generate(ctx context.Context, req report.ReportRequest) ([]byte, error)
}

type excelizeGenerator struct{}

// NewXLSXGenerator returns a stateless XLSXGenerator backed by Excelize v2.
func NewXLSXGenerator() XLSXGenerator {
	return &excelizeGenerator{}
}

// Generate produces an XLSX binary from req. Returns ErrInvalidFormat if
// req.OutputFormat is not FormatXLSX.
func (g *excelizeGenerator) Generate(_ context.Context, req report.ReportRequest) ([]byte, error) {
	if req.OutputFormat != report.FormatXLSX {
		return nil, report.ErrInvalidFormat
	}

	f := excelize.NewFile()
	defer f.Close() //nolint:errcheck

	sheet := "Sheet1"
	// Excelize creates Sheet1 by default; ensure index is correct.
	idx, _ := f.GetSheetIndex(sheet)
	f.SetActiveSheet(idx)

	// Headers
	if err := f.SetSheetRow(sheet, "A1", &[]string{"Field", "Value"}); err != nil {
		return nil, fmt.Errorf("xlsx header write failed: %w", err)
	}

	// Build data rows
	type kv struct{ k, v string }
	rows := []kv{
		{"Date From", req.Filters.DateFrom},
		{"Date To", req.Filters.DateTo},
	}
	if req.Filters.ClientID != nil {
		rows = append(rows, kv{"Client ID", fmt.Sprintf("%d", *req.Filters.ClientID)})
	}
	if req.Filters.EmployeeID != nil {
		rows = append(rows, kv{"Employee ID", *req.Filters.EmployeeID})
	}

	for i, r := range rows {
		cell := fmt.Sprintf("A%d", i+2)
		if err := f.SetSheetRow(sheet, cell, &[]string{r.k, r.v}); err != nil {
			return nil, fmt.Errorf("xlsx row write failed: %w", err)
		}
	}

	var buf bytes.Buffer
	if _, err := f.WriteTo(&buf); err != nil {
		return nil, fmt.Errorf("xlsx buffer write failed: %w", err)
	}

	return buf.Bytes(), nil
}
