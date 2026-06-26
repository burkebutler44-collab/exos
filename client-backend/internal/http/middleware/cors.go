package middleware

import (
	"net/http"
	"os"
	"strings"

	"github.com/gin-gonic/gin"
)

func CORS() gin.HandlerFunc {
	allowedOrigins := parseAllowedOrigins(os.Getenv("CORS_ALLOWED_ORIGINS"))
	if len(allowedOrigins) == 0 {
		allowedOrigins = map[string]bool{
			"http://localhost:5173": true,
			"http://localhost:5174": true,
			"http://127.0.0.1:5173": true,
			"http://127.0.0.1:5174": true,
		}
	}

	return func(c *gin.Context) {
		origin := c.GetHeader("Origin")
		if allowedOrigins["*"] || allowedOrigins[origin] {
			c.Header("Access-Control-Allow-Origin", origin)
			if allowedOrigins["*"] {
				c.Header("Access-Control-Allow-Origin", "*")
			}
			c.Header("Vary", "Origin")
			c.Header("Access-Control-Allow-Credentials", "true")
			c.Header("Access-Control-Allow-Headers", "Authorization, Content-Type, Accept")
			c.Header("Access-Control-Allow-Methods", "GET, POST, PATCH, DELETE, OPTIONS")
			c.Header("Access-Control-Max-Age", "600")
		}

		if c.Request.Method == http.MethodOptions {
			c.AbortWithStatus(http.StatusNoContent)
			return
		}

		c.Next()
	}
}

func parseAllowedOrigins(raw string) map[string]bool {
	origins := map[string]bool{}
	for _, item := range strings.Split(raw, ",") {
		origin := strings.TrimSpace(item)
		if origin != "" {
			origins[origin] = true
		}
	}
	return origins
}
