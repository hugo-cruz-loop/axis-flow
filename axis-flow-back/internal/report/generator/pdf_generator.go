// Package generator provides PDF and XLSX report generators.
package generator

import (
	"context"
	"fmt"

	"github.com/johnfercher/maroto/v2"
	"github.com/johnfercher/maroto/v2/pkg/components/col"
	"github.com/johnfercher/maroto/v2/pkg/components/row"
	"github.com/johnfercher/maroto/v2/pkg/components/text"
	"github.com/johnfercher/maroto/v2/pkg/config"
	"github.com/johnfercher/maroto/v2/pkg/props"

	report "axis-flow-back/internal/report"
)

// PDFGenerator generates report files in PDF format.
type PDFGenerator interface {
	Generate(ctx context.Context, req report.ReportRequest) ([]byte, error)
}

type marotoGenerator struct{}

// NewPDFGenerator returns a stateless PDFGenerator backed by Maroto v2.
func NewPDFGenerator() PDFGenerator {
	return &marotoGenerator{}
}

// Generate produces a PDF binary from req. Returns ErrInvalidFormat if
// req.OutputFormat is not FormatPDF.
func (g *marotoGenerator) Generate(_ context.Context, req report.ReportRequest) ([]byte, error) {
	if req.OutputFormat != report.FormatPDF {
		return nil, report.ErrInvalidFormat
	}

	cfg := config.NewBuilder().Build()
	m := maroto.New(cfg)

	// Title row
	m.AddRows(
		row.New(12).Add(
			col.New(12).Add(
				text.New(
					fmt.Sprintf("Report: %s | %s – %s", req.Type, req.Filters.DateFrom, req.Filters.DateTo),
					props.Text{Size: 14, Style: "B"},
				),
			),
		),
	)

	// Header row
	m.AddRows(
		row.New(8).Add(
			text.NewCol(6, "Field", props.Text{Style: "B"}),
			text.NewCol(6, "Value", props.Text{Style: "B"}),
		),
	)

	// Data rows for non-nil filter fields
	addDataRow := func(field, value string) {
		m.AddRows(
			row.New(6).Add(
				text.NewCol(6, field),
				text.NewCol(6, value),
			),
		)
	}

	addDataRow("Date From", req.Filters.DateFrom)
	addDataRow("Date To", req.Filters.DateTo)

	if req.Filters.ClientID != nil {
		addDataRow("Client ID", fmt.Sprintf("%d", *req.Filters.ClientID))
	}
	if req.Filters.EmployeeID != nil {
		addDataRow("Employee ID", *req.Filters.EmployeeID)
	}

	doc, err := m.Generate()
	if err != nil {
		return nil, fmt.Errorf("pdf generation failed: %w", err)
	}

	return doc.GetBytes(), nil
}
