// Package telemetry — metrics.go: Prometheus metrics for the
// Formularios PDF/S3 observability surface.
//
// PR-5 (5.3). The spec (§ Observabilidad) lists 5 metrics the
// formularios service must expose:
//
//   - formularios_pdf_render_duration_seconds (histogram)
//   - formularios_pdf_render_total            (counter, labels: status)
//   - formularios_s3_upload_duration_seconds  (histogram, labels: status)
//   - formularios_s3_upload_total             (counter, labels: status)
//   - formularios_s3_upload_bytes_total       (counter, labels: status)
//
// The label set is the minimum required to distinguish success
// from error: no PII (no path, filename, error text, user, email,
// nombre). The status label takes one of two values: "success" or
// "error". PR-6 (PDF/S3 Hardening) will add additional labels
// (reason=timeout|oom|exit_code) per the spec's failure-split
// table — kept in a separate PR so PR-5 stays focused on the
// observability SEAM, not the full label taxonomy.
//
// The constructor registers with the supplied prometheus.Registerer
// so the production main.go can wire the formularios metrics into
// the app-level registry (and so unit tests can use a fresh
// prometheus.NewRegistry() per case to avoid the duplicate-
// registration panic that promauto.With() would otherwise hit).
package telemetry

import "github.com/prometheus/client_golang/prometheus"

// Metrics holds the Prometheus collectors for the formularios
// service. The struct is intentionally small (5 metrics) so the
// service-layer wiring is trivial. Callers obtain an instance
// once at startup and inject it into the services that need it.
type Metrics struct {
	// PDF render latency (seconds) per render call. Buckets cover
	// the spec's "10s WKHTMLTOPDF_TIMEOUT_SECONDS" range with
	// headroom for slow VPS instances.
	PDFRenderDuration *prometheus.HistogramVec

	// PDF render call count, split by success/error.
	PDFRenderTotal *prometheus.CounterVec

	// S3 upload latency (seconds) per upload call.
	S3UploadDuration *prometheus.HistogramVec

	// S3 upload call count, split by success/error.
	S3UploadTotal *prometheus.CounterVec

	// S3 upload bytes total — useful for capacity planning and
	// the spec's "evidence photos + PDF reports" storage budget.
	S3UploadBytes *prometheus.CounterVec
}

// NewMetrics registers and returns the formularios Prometheus
// metrics. The caller owns the registry; subsequent calls with
// the same registry would panic (this is intentional — it catches
// re-wiring at startup). The PDF service / S3 wiring use the
// returned struct to record observations on the success and error
// paths.
//
// Label values are restricted to "success" or "error". Adding new
// status values is a breaking change for the dashboards — the
// PR-6 task adds the spec's "reason" sub-label (timeout / oom /
// exit_code) for the failure counters, not the status label.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	m := &Metrics{
		PDFRenderDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "formularios_pdf_render_duration_seconds",
			Help:    "Duration in seconds of wkhtmltopdf render calls.",
			Buckets: []float64{0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30, 60},
		}, []string{"status"}),
		PDFRenderTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "formularios_pdf_render_total",
			Help: "Total number of PDF render calls, split by status (success|error).",
		}, []string{"status"}),
		S3UploadDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "formularios_s3_upload_duration_seconds",
			Help:    "Duration in seconds of S3 upload calls for evidence and PDF reports.",
			Buckets: []float64{0.05, 0.1, 0.25, 0.5, 1, 2.5, 5, 10, 30},
		}, []string{"status"}),
		S3UploadTotal: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "formularios_s3_upload_total",
			Help: "Total number of S3 upload calls, split by status (success|error).",
		}, []string{"status"}),
		S3UploadBytes: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: "formularios_s3_upload_bytes_total",
			Help: "Total bytes uploaded to S3, split by status (success|error).",
		}, []string{"status"}),
	}
	// Register every collector. The Registerer panics on duplicate
	// registration; the production caller (main.go) invokes
	// NewMetrics exactly once at startup, so this is intentional.
	reg.MustRegister(
		m.PDFRenderDuration,
		m.PDFRenderTotal,
		m.S3UploadDuration,
		m.S3UploadTotal,
		m.S3UploadBytes,
	)
	return m
}
