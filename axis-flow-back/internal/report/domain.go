// Package report contains domain types and errors for the ReportBro report
// generation service.
package report

import "errors"

// OutputFormat enumerates supported report output formats.
type OutputFormat string

const (
	// FormatPDF produces a PDF binary output.
	FormatPDF OutputFormat = "pdf"
	// FormatXLSX produces an Excel spreadsheet output.
	FormatXLSX OutputFormat = "xlsx"
)

// ReportFilters carries the user-supplied filter values used when rendering
// a report template.
type ReportFilters struct {
	DateFrom   string `json:"date_from"`
	DateTo     string `json:"date_to"`
	ClientID   *int    `json:"client_id,omitempty"`
	EmployeeID *string `json:"employee_id,omitempty"`
}

// ReportRequest is the input to the report-generation use-case.
type ReportRequest struct {
	Type         string
	Filters      ReportFilters
	OutputFormat OutputFormat
	UserID       string
	EmpresaID    string
}

// ReportResult is the output of a successful report-generation use-case.
type ReportResult struct {
	Key         string
	DownloadURL string
	ExpiresAt   string
}

// Sentinel errors returned by the report domain and its use-cases.
var (
	// ErrInvalidFormat is returned when an unsupported OutputFormat is requested.
	ErrInvalidFormat = errors.New("report: invalid output format")
	// ErrReportNotFound is returned when a report key is absent or has expired.
	ErrReportNotFound = errors.New("report: report not found or expired")
	// ErrSignatureInvalid is returned when a download signature fails HMAC verification.
	ErrSignatureInvalid = errors.New("report: invalid or expired download signature")
)
