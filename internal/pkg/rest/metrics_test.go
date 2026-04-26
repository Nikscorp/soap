package rest

import (
	"net/http"
	"net/http/httptest"
	"testing"

	"github.com/go-chi/chi/v5"
	"github.com/prometheus/client_golang/prometheus"
	dto "github.com/prometheus/client_model/go"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"
)

func TestNewMetrics_RegistersAndCounts(t *testing.T) {
	prevReg := prometheus.DefaultRegisterer
	prevGather := prometheus.DefaultGatherer
	t.Cleanup(func() {
		prometheus.DefaultRegisterer = prevReg
		prometheus.DefaultGatherer = prevGather
	})
	reg := prometheus.NewRegistry()
	prometheus.DefaultRegisterer = reg
	prometheus.DefaultGatherer = reg

	m := NewMetrics()
	require.NotNil(t, m)

	r := chi.NewRouter()
	r.Use(m.Middleware)
	r.Get("/things/{id}", func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusCreated)
	})

	rec := httptest.NewRecorder()
	r.ServeHTTP(rec, httptest.NewRequest(http.MethodGet, "/things/7", nil))
	assert.Equal(t, http.StatusCreated, rec.Code)

	// Verify the counters were updated for the route pattern (not the raw path).
	mfs, err := reg.Gather()
	require.NoError(t, err)

	totals := findMetric(mfs, "http_requests_total")
	require.NotNil(t, totals, "http_requests_total not gathered")
	assert.True(t, hasLabel(totals, "path", "/things/{id}"))

	statuses := findMetric(mfs, "response_status")
	require.NotNil(t, statuses)
	assert.True(t, hasLabel(statuses, "status", "201"))
}

func TestNewMetrics_DuplicateRegistrationDoesNotPanic(t *testing.T) {
	prev := prometheus.DefaultRegisterer
	t.Cleanup(func() { prometheus.DefaultRegisterer = prev })
	prometheus.DefaultRegisterer = prometheus.NewRegistry()

	require.NotNil(t, NewMetrics())
	// Second call must hit the warn-and-continue branches without panicking.
	require.NotPanics(t, func() { NewMetrics() })
}

func findMetric(mfs []*dto.MetricFamily, name string) *dto.MetricFamily {
	for _, mf := range mfs {
		if mf.GetName() == name {
			return mf
		}
	}
	return nil
}

func hasLabel(mf *dto.MetricFamily, name, value string) bool {
	for _, m := range mf.GetMetric() {
		for _, l := range m.GetLabel() {
			if l.GetName() == name && l.GetValue() == value {
				return true
			}
		}
	}
	return false
}
