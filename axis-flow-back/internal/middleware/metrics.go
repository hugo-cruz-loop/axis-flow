package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

// Metrics holds all Prometheus metrics for the service.
type Metrics struct {
	RequestDuration *prometheus.HistogramVec
	RequestsTotal   *prometheus.CounterVec
	LoginSuccess    prometheus.Counter
	LoginFailure    prometheus.Counter
}

// NewMetrics registers and returns Prometheus metrics.
// Call once at startup and reuse the returned Metrics.
func NewMetrics(reg prometheus.Registerer) *Metrics {
	factory := promauto.With(reg)
	return &Metrics{
		RequestDuration: factory.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "http_request_duration_seconds",
			Help:    "Duration of HTTP requests in seconds.",
			Buckets: []float64{0.005, 0.01, 0.025, 0.05, 0.1, 0.25, 0.5, 1, 2.5},
		}, []string{"method", "path", "status"}),
		RequestsTotal: factory.NewCounterVec(prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Total number of HTTP requests.",
		}, []string{"method", "path", "status"}),
		LoginSuccess: factory.NewCounter(prometheus.CounterOpts{
			Name: "auth_login_success_total",
			Help: "Total number of successful login attempts.",
		}),
		LoginFailure: factory.NewCounter(prometheus.CounterOpts{
			Name: "auth_login_failure_total",
			Help: "Total number of failed login attempts.",
		}),
	}
}

// MetricsMiddleware returns a chi-compatible middleware that records request
// duration and total request count per method/path/status.
func MetricsMiddleware(m *Metrics) func(http.Handler) http.Handler {
	return func(next http.Handler) http.Handler {
		return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
			start := time.Now()
			wrapped := &responseWriter{ResponseWriter: w, status: http.StatusOK}

			next.ServeHTTP(wrapped, r)

			duration := time.Since(start).Seconds()
			statusStr := strconv.Itoa(wrapped.status)

			m.RequestDuration.WithLabelValues(r.Method, r.URL.Path, statusStr).Observe(duration)
			m.RequestsTotal.WithLabelValues(r.Method, r.URL.Path, statusStr).Inc()
		})
	}
}
