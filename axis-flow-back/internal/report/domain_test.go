package report_test

import (
	"testing"

	"axis-flow-back/internal/report"
)

func TestOutputFormatValues(t *testing.T) {
	if report.FormatPDF != "pdf" {
		t.Errorf("FormatPDF = %q; want %q", report.FormatPDF, "pdf")
	}
	if report.FormatXLSX != "xlsx" {
		t.Errorf("FormatXLSX = %q; want %q", report.FormatXLSX, "xlsx")
	}
}

func TestSentinelErrorsAreDistinct(t *testing.T) {
	errs := []error{
		report.ErrInvalidFormat,
		report.ErrReportNotFound,
		report.ErrSignatureInvalid,
	}
	for i := 0; i < len(errs); i++ {
		for j := i + 1; j < len(errs); j++ {
			if errs[i] == errs[j] {
				t.Errorf("error[%d] == error[%d]: both are %v", i, j, errs[i])
			}
		}
	}
}
