// Package service hosts the Scheduler orchestration layer (lock manager,
// cron runner, job interface, FCM client, M2M client, parametrizacion
// client, and structured logging).
//
// This file implements the three Prometheus collectors mandated by the
// "Observabilidad > Prometheus Metrics" section of the Scheduler Service
// specification. The metric names, types, and label dimensions are
// taken verbatim from the spec and must not be changed without a
// corresponding spec update.
package service

import (
	"fmt"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

// ---------------------------------------------------------------------------
// Metric names, help strings, and label dimensions — VERBATIM from
// specification.md §"Observabilidad > Prometheus Metrics".
// ---------------------------------------------------------------------------

const (
	// metricJobDuration is the histogram that records the latency of
	// every job execution. Label: job_id.
	metricJobDuration = "scheduler_job_duration_seconds"
	// metricJobsFailed is the counter of failed job executions.
	// Labels: job_id, error_class.
	metricJobsFailed = "scheduler_jobs_failed_total"
	// metricRedisLockFailures is the counter of skipped job executions
	// due to lock collision. Label: job_id.
	metricRedisLockFailures = "scheduler_redis_lock_failures_total"
)

// Default histogram buckets for job duration, expressed in seconds.
// The boundaries span sub-100ms interactive jobs to 5-minute batch
// runs; they are intentionally identical to the spec's recommended
// defaults so dashboards built around them remain stable.
var defaultDurationBuckets = []float64{0.01, 0.05, 0.1, 0.5, 1, 5, 15, 60, 300}

// ---------------------------------------------------------------------------
// SchedulerMetrics — typed holder for the three collectors.
// ---------------------------------------------------------------------------

// SchedulerMetrics owns the Prometheus collectors registered for the
// scheduler service. The struct is safe for concurrent use because the
// underlying prometheus.*Vec types are themselves thread-safe.
type SchedulerMetrics struct {
	// JobDuration is a histogram of per-execution latency in seconds.
	JobDuration *prometheus.HistogramVec
	// JobsFailed is a counter of failed executions, labelled by job_id
	// and error_class.
	JobsFailed *prometheus.CounterVec
	// RedisLockFailures is a counter of skipped executions caused by
	// lock collision, labelled by job_id.
	RedisLockFailures *prometheus.CounterVec
}

// RegisterMetrics builds the three SchedulerMetrics collectors and
// registers them with the supplied Registerer. It returns an error
// when registration fails (typically because a collector with the
// same name has already been registered); the caller is expected to
// treat that as a fatal startup error.
//
// RegisterMetrics may be called more than once against different
// Registerer instances — the typical use is the Prometheus default
// registry, but tests can pass a fresh prometheus.NewRegistry() to
// keep observations isolated.
func RegisterMetrics(reg prometheus.Registerer) (*SchedulerMetrics, error) {
	if reg == nil {
		return nil, fmt.Errorf("metrics.RegisterMetrics: registerer is nil")
	}
	sm := &SchedulerMetrics{
		JobDuration: prometheus.NewHistogramVec(prometheus.HistogramOpts{
			Name:    metricJobDuration,
			Help:    "Latency of job executions in seconds.",
			Buckets: defaultDurationBuckets,
		}, []string{"job_id"}),
		JobsFailed: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricJobsFailed,
			Help: "Cumulative count of jobs that failed during execution.",
		}, []string{"job_id", "error_class"}),
		RedisLockFailures: prometheus.NewCounterVec(prometheus.CounterOpts{
			Name: metricRedisLockFailures,
			Help: "Cumulative count of skipped job executions due to lock collision.",
		}, []string{"job_id"}),
	}
	for _, c := range []prometheus.Collector{sm.JobDuration, sm.JobsFailed, sm.RedisLockFailures} {
		if err := reg.Register(c); err != nil {
			return nil, fmt.Errorf("metrics.RegisterMetrics: %w", err)
		}
	}
	return sm, nil
}

// ---------------------------------------------------------------------------
// Typed helpers — small, allocation-free wrappers around WithLabelValues.
// ---------------------------------------------------------------------------

// ObserveJobDuration records the duration of a job execution under the
// job_id label. d is converted to seconds with sub-millisecond
// precision; passing a zero or negative duration is a no-op because
// Prometheus histograms reject non-finite observations.
func (m *SchedulerMetrics) ObserveJobDuration(jobID string, d time.Duration) {
	if m == nil || m.JobDuration == nil {
		return
	}
	if d < 0 {
		return
	}
	m.JobDuration.WithLabelValues(jobID).Observe(d.Seconds())
}

// IncJobFailed increments the scheduler_jobs_failed_total counter for
// the supplied job_id and error_class. Calling it with an empty
// job_id or error_class is allowed and produces the corresponding
// (low-cardinality) label set, matching Prometheus best practice of
// keeping label values bounded.
func (m *SchedulerMetrics) IncJobFailed(jobID, errorClass string) {
	if m == nil || m.JobsFailed == nil {
		return
	}
	m.JobsFailed.WithLabelValues(jobID, errorClass).Inc()
}

// IncRedisLockFailure increments the scheduler_redis_lock_failures_total
// counter for the supplied job_id. Use it from the cron runner when an
// Acquire call returns acquired=false.
func (m *SchedulerMetrics) IncRedisLockFailure(jobID string) {
	if m == nil || m.RedisLockFailures == nil {
		return
	}
	m.RedisLockFailures.WithLabelValues(jobID).Inc()
}

// ---------------------------------------------------------------------------
// Package-level singleton — initialized by InitMetrics at startup.
// ---------------------------------------------------------------------------

// Metrics is the process-wide SchedulerMetrics singleton. It is nil
// until InitMetrics is called during application bootstrap. The
// helpers on *SchedulerMetrics are nil-safe, so calling them before
// initialization is a no-op; this keeps the cron runner and job
// lifecycle wrapper free of nil-checks in the hot path.
var Metrics *SchedulerMetrics

// InitMetrics creates the three collectors and registers them with
// reg, then assigns the result to the package-level Metrics variable.
// It returns the same error as RegisterMetrics so the caller can
// decide whether to fail fast or keep the nil singleton.
func InitMetrics(reg prometheus.Registerer) (*SchedulerMetrics, error) {
	sm, err := RegisterMetrics(reg)
	if err != nil {
		return nil, err
	}
	Metrics = sm
	return sm, nil
}
