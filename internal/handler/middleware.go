package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"
)

func CORSMiddleware(origin string) gin.HandlerFunc {
	return func(c *gin.Context) {
		if origin == "" {
			origin = "*"
		}
		c.Header("Access-Control-Allow-Origin", origin)
		c.Next()
	}
}

func BodySizeMiddleware(maxBytes int64) gin.HandlerFunc {
	return func(c *gin.Context) {
		if maxBytes <= 0 {
			maxBytes = 1 << 20 // 1 MB default
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		c.Next()
	}
}