package rest

import (
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/prometheus/client_golang/prometheus"
)

type Metrics struct {
	totalRequests  *prometheus.CounterVec
	responseStatus *prometheus.CounterVec
	httpDuration   *prometheus.HistogramVec
	pathSet        map[string]struct{}
}

func NewMetrics(paths []string) *Metrics {
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
		log.Printf("[WARN] can't register prometheus totalRequests: %v", err)
	}
	if err := prometheus.Register(res.responseStatus); err != nil {
		log.Printf("[WARN] can't register prometheus responseStatus: %v", err)
	}
	if err := prometheus.Register(res.httpDuration); err != nil {
		log.Printf("[WARN] can't register prometheus httpDuration: %v", err)
	}

	pathSet := make(map[string]struct{}, len(paths))
	for _, path := range paths {
		pathSet[strings.ToLower(path)] = struct{}{}
	}
	res.pathSet = pathSet

	return res
}

func (m *Metrics) Middleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		splitted := strings.Split(r.URL.Path, "/")
		path := splitted[0]
		if path == "" && len(splitted) > 1 {
			path = splitted[1]
		}
		path = strings.ToLower(path)

		if _, ok := m.pathSet[path]; !ok {
			next.ServeHTTP(w, r)
			return
		}

		m.totalRequests.WithLabelValues(path).Inc()

		rw := NewResponseWriter(w)
		start := time.Now()

		next.ServeHTTP(rw, r)

		statusCodeStr := strconv.Itoa(rw.statusCode)
		m.httpDuration.WithLabelValues(path, statusCodeStr).Observe(float64(time.Since(start)) / float64(time.Second))
		m.responseStatus.WithLabelValues(path, statusCodeStr).Inc()
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func NewResponseWriter(w http.ResponseWriter) *responseWriter { //nolint
	return &responseWriter{w, http.StatusOK}
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}
