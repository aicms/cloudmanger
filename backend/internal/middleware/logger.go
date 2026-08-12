package middleware

import (
	"log"
	"time"

	"github.com/gin-gonic/gin"
)

// Logger 简单请求日志
func Logger() gin.HandlerFunc {
	return func(c *gin.Context) {
		start := time.Now()
		c.Next()
		latency := time.Since(start)
		log.Printf("[API] %s %s %d %v",
			c.Request.Method, c.Request.URL.Path, c.Writer.Status(), latency)
	}
}
