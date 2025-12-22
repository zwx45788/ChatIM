package middleware

import (
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"ChatIM/pkg/metrics"
)

// PrometheusMiddleware Prometheus 指标采集中间件
func PrometheusMiddleware() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		path := c.FullPath()
		if path == "" {
			path = c.Request.URL.Path
		}

		// 记录请求大小
		if c.Request.ContentLength > 0 {
			metrics.HttpRequestSizeBytes.WithLabelValues(
				c.Request.Method,
				path,
			).Observe(float64(c.Request.ContentLength))
		}

		// 执行请求
		c.Next()

		// 计算延迟
		duration := time.Since(start).Seconds()
		status := strconv.Itoa(c.Writer.Status())

		// 记录 HTTP 请求总数
		metrics.HttpRequestsTotal.WithLabelValues(
			c.Request.Method,
			path,
			status,
		).Inc()

		// 记录请求延迟
		metrics.HttpRequestDuration.WithLabelValues(
			c.Request.Method,
			path,
		).Observe(duration)

		// 记录响应大小
		if c.Writer.Size() > 0 {
			metrics.HttpResponseSizeBytes.WithLabelValues(
				c.Request.Method,
				path,
			).Observe(float64(c.Writer.Size()))
		}
	}
}
