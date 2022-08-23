package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"github.com/gopherchai/contrib/lib/metrics"
)

const (
	NSDemo = "ns_demo"
)

var (
	requestCounter     = metrics.NewCounterVec(NSDemo, "httpServer", "requestCounter", "request_total", []string{"method", "path", "status"})
	requestDurationObs = metrics.NewHistogramVec(NSDemo, "httpServer", "requestDuration", "help", []string{"method", "path"})
	//responseSizeHistogram=metrics.
)

func Metrics() gin.HandlerFunc {
	return func(c *gin.Context) {
		startAt := time.Now()
		c.Next()
		r := c.Request
		w := c.Writer
		requestCounter.Inc(r.Method, r.URL.Path, strconv.Itoa(w.Status()))
		d := time.Now().Sub(startAt).Milliseconds()
		requestDurationObs.Observe(float64(d), r.Method, r.URL.Path)
	}
}
