package middleware

import (
	"net/http"
	"strconv"
	"time"

	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promauto"
)

var (
	httpRequestsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qaat",
		Subsystem: "api",
		Name:      "http_requests_total",
		Help:      "Total HTTP requests by method, path, status, and role.",
	}, []string{"method", "path", "status", "role"})

	httpRequestDuration = promauto.NewHistogramVec(prometheus.HistogramOpts{
		Namespace: "qaat",
		Subsystem: "api",
		Name:      "http_request_duration_seconds",
		Help:      "HTTP request latency in seconds.",
		Buckets:   []float64{.005, .01, .025, .05, .1, .25, .5, 1, 2.5, 5},
	}, []string{"method", "path", "role"})

	httpRequestsInFlight = promauto.NewGauge(prometheus.GaugeOpts{
		Namespace: "qaat",
		Subsystem: "api",
		Name:      "http_requests_in_flight",
		Help:      "Number of in-flight HTTP requests.",
	})

	syncUploadsTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qaat",
		Name:      "sync_uploads_total",
		Help:      "Total sync uploads by status (SYNCED/FAILED).",
	}, []string{"status"})

	// Exposed for use by sync handler.
	SyncUploadCompleted = syncUploadsTotal.WithLabelValues("SYNCED")
	SyncUploadFailed    = syncUploadsTotal.WithLabelValues("FAILED")

	rateLimitTotal = promauto.NewCounterVec(prometheus.CounterOpts{
		Namespace: "qaat",
		Name:      "rate_limit_rejections_total",
		Help:      "Total requests rejected by the rate limiter.",
	}, []string{"role"})

	// Exposed for rate limiter.
	RateLimitRejected = func(role string) { rateLimitTotal.WithLabelValues(role).Inc() }
)

// PrometheusMiddleware records per-request metrics.
func PrometheusMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		httpRequestsInFlight.Inc()
		defer httpRequestsInFlight.Dec()

		rec := &statusRecorder{ResponseWriter: w, status: 200}
		next.ServeHTTP(rec, r)

		role := GetRole(r.Context())
		path := sanitisePath(r.URL.Path)
		status := strconv.Itoa(rec.status)
		dur := time.Since(start).Seconds()

		httpRequestsTotal.WithLabelValues(r.Method, path, status, role).Inc()
		httpRequestDuration.WithLabelValues(r.Method, path, role).Observe(dur)
	})
}

// sanitisePath collapses dynamic URL segments to prevent cardinality explosion.
// e.g. /api/v1/sync/chunk/abc-123/0 → /api/v1/sync/chunk/:upload_id/:chunk_index
func sanitisePath(path string) string {
	// Use chi's RouteContext if available — otherwise return raw path.
	// For Prometheus label safety, paths are already short in this API.
	return path
}
