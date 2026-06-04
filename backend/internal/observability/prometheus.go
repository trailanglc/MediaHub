package observability

import (
	"strconv"
	"sync/atomic"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
	"github.com/prometheus/client_golang/prometheus/promhttp"
)

var (
	httpRequestsTotal = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mediahub_http_requests_total",
			Help: "Total HTTP requests handled by the API.",
		},
		[]string{"method", "path", "status"},
	)
	httpRequestDuration = prometheus.NewHistogramVec(
		prometheus.HistogramOpts{
			Name:    "mediahub_http_request_duration_seconds",
			Help:    "HTTP request latency in seconds.",
			Buckets: prometheus.DefBuckets,
		},
		[]string{"method", "path"},
	)
	apiStartedUnix = time.Now().Unix()
)

func init() {
	prometheus.MustRegister(httpRequestsTotal, httpRequestDuration)
}

// PrometheusHandler serves standard Prometheus metrics (protected routes should wrap externally if needed).
func PrometheusHandler() gin.HandlerFunc {
	h := promhttp.Handler()
	return func(c *gin.Context) {
		h.ServeHTTP(c.Writer, c.Request)
	}
}

// HTTPMetricsMiddleware records request counts and latency for Prometheus.
func HTTPMetricsMiddleware() gin.HandlerFunc {
	var inFlight int64
	return func(c *gin.Context) {
		start := time.Now()
		atomic.AddInt64(&inFlight, 1)
		c.Next()
		atomic.AddInt64(&inFlight, -1)
		status := strconv.Itoa(c.Writer.Status())
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}
		httpRequestsTotal.WithLabelValues(c.Request.Method, path, status).Inc()
		httpRequestDuration.WithLabelValues(c.Request.Method, path).Observe(time.Since(start).Seconds())
	}
}

// APIUptimeSeconds returns seconds since process start for health payloads.
func APIUptimeSeconds() float64 {
	return time.Since(time.Unix(apiStartedUnix, 0)).Seconds()
}
