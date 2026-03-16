package monitoring

import (
	"bufio"
	"fmt"
	"net"
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

type Metrics struct {
	HTTPRequestsTotal  *prometheus.CounterVec
	HTTPRequestTime    *prometheus.HistogramVec
	DeploymentsCreated prometheus.Counter
}

func NewMetrics() *Metrics {
	return &Metrics{
		HTTPRequestsTotal: promauto.NewCounterVec(prometheus.CounterOpts{
			Name: "instantdeploy_http_requests_total",
			Help: "Total HTTP requests",
		}, []string{"method", "path", "status"}),
		HTTPRequestTime: promauto.NewHistogramVec(prometheus.HistogramOpts{
			Name:    "instantdeploy_http_request_duration_seconds",
			Help:    "HTTP request latency",
			Buckets: prometheus.DefBuckets,
		}, []string{"method", "path"}),
		DeploymentsCreated: promauto.NewCounter(prometheus.CounterOpts{
			Name: "instantdeploy_deployments_created_total",
			Help: "Total deployments created",
		}),
	}
}

func (m *Metrics) HTTPMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		rw := &responseWriter{ResponseWriter: w, statusCode: http.StatusOK}
		next.ServeHTTP(rw, r)
		path := r.URL.Path
		status := strconv.Itoa(rw.statusCode)
		m.HTTPRequestsTotal.WithLabelValues(r.Method, path, status).Inc()
		m.HTTPRequestTime.WithLabelValues(r.Method, path).Observe(time.Since(start).Seconds())
	})
}

type responseWriter struct {
	http.ResponseWriter
	statusCode int
}

func (rw *responseWriter) WriteHeader(code int) {
	rw.statusCode = code
	rw.ResponseWriter.WriteHeader(code)
}

func (rw *responseWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	hj, ok := rw.ResponseWriter.(http.Hijacker)
	if !ok {
		return nil, nil, fmt.Errorf("response writer does not support hijacking")
	}
	return hj.Hijack()
}

func (rw *responseWriter) Flush() {
	if f, ok := rw.ResponseWriter.(http.Flusher); ok {
		f.Flush()
	}
}
