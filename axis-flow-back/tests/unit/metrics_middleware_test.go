package unit_test

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"axis-flow-back/internal/middleware"

	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestMetricsMiddleware_RecordsRequestDuration(t *testing.T) {
	reg := prometheus.NewRegistry()
	m := middleware.NewMetrics(reg)

	mw := middleware.MetricsMiddleware(m)
	handler := mw(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))

	req := httptest.NewRequest(http.MethodGet, "/health/live", nil)
	rec := httptest.NewRecorder()
	handler.ServeHTTP(rec, req)

	// Gather all metrics.
	mfs, err := reg.Gather()
	require.NoError(t, err)

	var histFamily *dto.MetricFamily
	for _, mf := range mfs {
		if mf.GetName() == "http_request_duration_seconds" {
			histFamily = mf
			break
		}
	}

	require.NotNil(t, histFamily, "expected http_request_duration_seconds metric family")
	require.NotEmpty(t, histFamily.GetMetric(), "expected at least one histogram metric")

	hist := histFamily.GetMetric()[0].GetHistogram()
	assert.Equal(t, uint64(1), hist.GetSampleCount(), "expected exactly one observation")
}
