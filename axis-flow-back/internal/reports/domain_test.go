// Package reports_test contains black-box tests for the reports domain types.
// Using external test package to verify the public API surface.
package reports_test

import (
	"testing"

	"axis-flow-back/internal/reports"

	"github.com/stretchr/testify/assert"
)

// TestMaskEmpleadoName verifies the PII masking helper across all documented
// edge cases. The function must return "First_initial. LastName" for
// multi-word names, and degrade gracefully for single-word or empty input.
func TestMaskEmpleadoName(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name     string
		input    string
		expected string
	}{
		{
			name:     "two-word name",
			input:    "Juan Pérez",
			expected: "J. Pérez",
		},
		{
			name:     "multi-word name — first initial + last word",
			input:    "María de los Ángeles García",
			expected: "M. García",
		},
		{
			name:     "single-word name — graceful",
			input:    "Carlos",
			expected: "C.",
		},
		{
			name:     "empty string — no-op",
			input:    "",
			expected: "",
		},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			got := reports.MaskEmpleadoName(tc.input)
			assert.Equal(t, tc.expected, got)
		})
	}
}

// TestSentinelErrorsAreDistinct asserts that all sentinel errors are unique
// values — callers that use errors.Is must be able to distinguish them.
func TestSentinelErrorsAreDistinct(t *testing.T) {
	t.Parallel()

	sentinels := []error{
		reports.ErrNotFound,
		reports.ErrForbidden,
		reports.ErrRateLimitExceeded,
		reports.ErrGeocodingUnavailable,
	}

	for i := 0; i < len(sentinels); i++ {
		for j := i + 1; j < len(sentinels); j++ {
			assert.NotEqual(t, sentinels[i], sentinels[j],
				"sentinels at index %d and %d must be distinct", i, j)
		}
	}
}
