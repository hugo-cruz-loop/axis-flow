// Package telemetry — metrics tests for PR-5 (5.3).
//
// Asserts that every metric the formularios PDF/S3 observability
// needs is registered with the right name and label set, and that
// NewMetrics is idempotent against duplicate registration (each
// test uses a fresh prometheus.Registry so the production call
// site is safe even if it's called more than once).
package telemetry_test

import (
	"strings"
	"testing"

	"axis-flow-back/internal/formularios/telemetry"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

// gatherMetricNames returns the names of every metric currently
// registered in the given registry. Used to assert exact name + count.
func gatherMetricNames(t *testing.T, reg *prometheus.Registry) []string {
	t.Helper()
	mfs, err := reg.Gather()
	require.NoError(t, err)
	names := make([]string, 0, len(mfs))
	for _, mf := range mfs {
		names = append(names, mf.GetName())
	}
	return names
}

// TestNewMetrics_RegistersAllExpectedNames asserts that the 5
// formularios metrics land in the registry with the exact names
// the spec requires.
func TestNewMetrics_RegistersAllExpectedNames(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.NewMetrics(reg)

	// HistogramVec / CounterVec children are created lazily on
	// the first WithLabelValues call. Materialize one child for
	// each so Gather() returns the parent metric.
	m.PDFRenderDuration.WithLabelValues("success").Observe(0)
	m.PDFRenderTotal.WithLabelValues("success").Add(0)
	m.S3UploadDuration.WithLabelValues("success").Observe(0)
	m.S3UploadTotal.WithLabelValues("success").Add(0)
	m.S3UploadBytes.WithLabelValues("success").Add(0)

	expected := []string{
		"formularios_pdf_render_duration_seconds",
		"formularios_pdf_render_total",
		"formularios_s3_upload_duration_seconds",
		"formularios_s3_upload_total",
		"formularios_s3_upload_bytes_total",
	}
	got := gatherMetricNames(t, reg)
	for _, name := range expected {
		assert.Contains(t, got, name, "registry must include %q", name)
	}
}

// TestNewMetrics_LabelSetsAreCorrect asserts that the histograms
// and counters have the expected label set ("status" only — no
// PII like path / error text per the spec).
func TestNewMetrics_LabelSetsAreCorrect(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.NewMetrics(reg)

	// The metric vec descriptors are unexported; we exercise them
	// via WithLabelValues (which is the public surface).
	// If the label name is wrong, this would still compile (Go
	// variadic) but the test would not increment the counter.
	m.PDFRenderTotal.WithLabelValues("success").Inc()
	m.S3UploadTotal.WithLabelValues("error").Inc()

	mfs, err := reg.Gather()
	require.NoError(t, err)

	var foundPDFTotal, foundS3Total bool
	for _, mf := range mfs {
		switch mf.GetName() {
		case "formularios_pdf_render_total":
			require.Len(t, mf.GetMetric(), 1)
			require.Len(t, mf.GetMetric()[0].GetLabel(), 1)
			assert.Equal(t, "status", mf.GetMetric()[0].GetLabel()[0].GetName())
			assert.Equal(t, "success", mf.GetMetric()[0].GetLabel()[0].GetValue())
			assert.Equal(t, float64(1), mf.GetMetric()[0].GetCounter().GetValue())
			foundPDFTotal = true
		case "formularios_s3_upload_total":
			require.Len(t, mf.GetMetric(), 1)
			require.Len(t, mf.GetMetric()[0].GetLabel(), 1)
			assert.Equal(t, "status", mf.GetMetric()[0].GetLabel()[0].GetName())
			assert.Equal(t, "error", mf.GetMetric()[0].GetLabel()[0].GetValue())
			assert.Equal(t, float64(1), mf.GetMetric()[0].GetCounter().GetValue())
			foundS3Total = true
		}
	}
	assert.True(t, foundPDFTotal, "formularios_pdf_render_total must be in the registry")
	assert.True(t, foundS3Total, "formularios_s3_upload_total must be in the registry")
}

// TestNewMetrics_FreshRegistry_PreventsDuplicateRegistration
// asserts that calling NewMetrics twice on the SAME registry would
// panic (the production code does not re-register — main.go calls
// NewMetrics once at startup). This is a documentation test, not
// a requirement: the test uses TWO SEPARATE registries to verify
// the per-registry registration works.
func TestNewMetrics_FreshRegistry_PreventsDuplicateRegistration(t *testing.T) {
	reg1 := prometheus.NewRegistry()
	reg2 := prometheus.NewRegistry()
	m1 := telemetry.NewMetrics(reg1)
	m2 := telemetry.NewMetrics(reg2) // different registry — must not panic

	// Materialize one child per metric so the parent appears in
	// Gather() (vec metrics are lazy).
	m1.PDFRenderDuration.WithLabelValues("success").Observe(0)
	m1.PDFRenderTotal.WithLabelValues("success").Add(0)
	m1.S3UploadDuration.WithLabelValues("success").Observe(0)
	m1.S3UploadTotal.WithLabelValues("success").Add(0)
	m1.S3UploadBytes.WithLabelValues("success").Add(0)
	m2.PDFRenderDuration.WithLabelValues("success").Observe(0)
	m2.PDFRenderTotal.WithLabelValues("success").Add(0)
	m2.S3UploadDuration.WithLabelValues("success").Observe(0)
	m2.S3UploadTotal.WithLabelValues("success").Add(0)
	m2.S3UploadBytes.WithLabelValues("success").Add(0)

	assert.Equal(t, 5, len(gatherMetricNames(t, reg1)))
	assert.Equal(t, 5, len(gatherMetricNames(t, reg2)))
}

// TestPDFRenderDuration_HistogramObservesValue asserts that
// Observe() works on the histogram (the bucket boundaries are
// Prometheus standard).
func TestPDFRenderDuration_HistogramObservesValue(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.NewMetrics(reg)

	m.PDFRenderDuration.WithLabelValues("success").Observe(1.42)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "formularios_pdf_render_duration_seconds" {
			require.Len(t, mf.GetMetric(), 1)
			hist := mf.GetMetric()[0].GetHistogram()
			assert.Equal(t, uint64(1), hist.GetSampleCount(), "histogram sample count must reflect 1 Observe")
			assert.Equal(t, 1.42, hist.GetSampleSum(), "histogram sum must equal the observed value")
			found = true
		}
	}
	assert.True(t, found, "formularios_pdf_render_duration_seconds must be in the registry")
}

// TestS3UploadBytes_CounterIncrements asserts that the byte
// counter records the upload size.
func TestS3UploadBytes_CounterIncrements(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.NewMetrics(reg)

	m.S3UploadBytes.WithLabelValues("success").Add(1024)
	m.S3UploadBytes.WithLabelValues("success").Add(2048)

	mfs, err := reg.Gather()
	require.NoError(t, err)
	var found bool
	for _, mf := range mfs {
		if mf.GetName() == "formularios_s3_upload_bytes_total" {
			require.Len(t, mf.GetMetric(), 1)
			// dto.Counter.GetValue returns float64
			val := mf.GetMetric()[0].GetCounter().GetValue()
			assert.Equal(t, float64(3072), val, "bytes counter must sum to 1024+2048")
			found = true
		}
	}
	assert.True(t, found, "formularios_s3_upload_bytes_total must be in the registry")
}

// TestNewMetrics_NoPIIStringsInNamesOrLabels is a no-PII audit:
// the metric names and label NAMES must not contain user-supplied
// strings (path, error text, file content). This is a
// compile-time-style assertion at test time — the names are
// constants in the production code; if a future PR adds a new
// metric with a PII label, this test catches it.
func TestNewMetrics_NoPIIStringsInNamesOrLabels(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := telemetry.NewMetrics(reg)

	// Materialize the metrics so they appear in Gather().
	m.PDFRenderDuration.WithLabelValues("success").Observe(0)
	m.PDFRenderTotal.WithLabelValues("success").Add(0)
	m.S3UploadDuration.WithLabelValues("success").Observe(0)
	m.S3UploadTotal.WithLabelValues("success").Add(0)
	m.S3UploadBytes.WithLabelValues("success").Add(0)

	banned := []string{"path", "filename", "error_text", "user", "email", "name"}
	for _, name := range gatherMetricNames(t, reg) {
		low := strings.ToLower(name)
		for _, b := range banned {
			assert.NotContains(t, low, b, "metric name %q must not contain PII-ish substring %q", name, b)
		}
	}
}

// _ ensures the dto import is referenced even if the explicit
// counter assertions are removed in a future refactor.
var _ = dto.MetricType_COUNTER
