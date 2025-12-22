package middleware

import (
	"net/http"
	"strconv"
	"strings"

	"github.com/gin-gonic/gin"
)

// CORSMiddleware handles CORS for local development, including file:// pages (Origin: null).
func CORSMiddleware() gin.HandlerFunc {
	const maxAgeSeconds = 12 * 60 * 60

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if origin == "" {
			// Not a CORS request.
			c.Next()
			return
		}

		allowedOrigin, allowCredentials, ok := resolveCORS(origin)
		if !ok {
			if c.Request.Method == http.MethodOptions {
				c.AbortWithStatus(http.StatusForbidden)
				return
			}
			c.Next()
			return
		}

		c.Header("Access-Control-Allow-Origin", allowedOrigin)
		c.Header("Vary", "Origin")
		c.Header("Access-Control-Allow-Methods", "GET,POST,PUT,DELETE,OPTIONS")
		c.Header("Access-Control-Allow-Headers", "Origin, Content-Type, Authorization, Accept, X-Requested-With")
		c.Header("Access-Control-Expose-Headers", "Content-Length")
		c.Header("Access-Control-Max-Age", strconv.Itoa(maxAgeSeconds))
		if allowCredentials {
			c.Header("Access-Control-Allow-Credentials", "true")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func resolveCORS(origin string) (allowedOrigin string, allowCredentials bool, ok bool) {
	// file:// pages will send Origin: null
	if origin == "null" {
		return "null", false, true
	}

	// Allow local dev http/https origins on any port.
	if strings.HasPrefix(origin, "http://localhost:") || strings.HasPrefix(origin, "http://127.0.0.1:") ||
		strings.HasPrefix(origin, "https://localhost:") || strings.HasPrefix(origin, "https://127.0.0.1:") {
		return origin, true, true
	}

	// Allow local dev without explicit port.
	if origin == "http://localhost" || origin == "http://127.0.0.1" ||
		origin == "https://localhost" || origin == "https://127.0.0.1" {
		return origin, true, true
	}

	// Add production domains here if needed.
	return "", false, false
}
