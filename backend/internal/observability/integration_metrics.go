package observability

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/prometheus/client_golang/prometheus"
)

var (
	integrationRequests = prometheus.NewCounterVec(
		prometheus.CounterOpts{
			Name: "mediahub_integration_requests_total",
			Help: "Total integration API and asset delivery requests",
		},
		[]string{"route", "status"},
	)
)

func init() {
	prometheus.MustRegister(integrationRequests)
}

// IntegrationMetricsMiddleware records async-friendly counters for /api/v1 and /assets routes.
func IntegrationMetricsMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		route := c.FullPath()
		if route == "" {
			route = c.Request.URL.Path
		}
		status := strconv.Itoa(c.Writer.Status())
		integrationRequests.WithLabelValues(route, status).Inc()
		_ = start
	}
}
