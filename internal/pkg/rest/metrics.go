package rest

import (
	"context"
	"net/http"
	"strconv"
	"time"

	"github.com/Nikscorp/soap/internal/pkg/logger"
	"github.com/go-chi/chi/v5/middleware"
	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	totalRequests  *prometheus.CounterVec
	responseStatus *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
}

func NewMetrics() *Metrics {
	res := &Metrics{}

	res.totalRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "http_requests_total",
			Help: "Number of incoming requests.",
		},
		[]string{"path"},
	)

	res.responseStatus = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "response_status",
			Help: "Status of HTTP responses.",
		},
		[]string{"path", "status"},
	)

	res.httpDuration = prometheus.NewHistogramVec(prometheus.HistogramOpts{
		Name:    "http_response_time_seconds",
		Help:    "Duration of HTTP requests.",
		Buckets: []float64{0.005, 0.01, 0.1, 0.5, 1, 2, 3, 5},
	}, []string{"path", "status"})

	if err := prometheus.Register(res.totalRequests); err != nil {
		logger.Warn(context.Background(), "Can't register prometheus totalRequests", "err", err)
	}
	if err := prometheus.Register(res.responseStatus); err != nil {
		logger.Warn(context.Background(), "Can't register prometheus responseStatus", "err", err)
	}
	if err := prometheus.Register(res.httpDuration); err != nil {
		logger.Warn(context.Background(), "Can't register prometheus httpDuration", "err", err)
	}

	return res
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := routePattern(r)

		m.totalRequests.WithLabelValues(path).Inc()

		rw := middleware.NewWrapResponseWriter(w, r.ProtoMajor)
		start := time.Now()

		next.ServeHTTP(rw, r)

		statusCodeStr := strconv.Itoa(rw.Status())
		m.httpDuration.WithLabelValues(path, statusCodeStr).Observe(float64(time.Since(start)) / float64(time.Second))
		m.responseStatus.WithLabelValues(path, statusCodeStr).Inc()
	})
}
