package handler

import (
	"github.com/gin-gonic/gin"
)

// writeJSON menulis response JSON dengan status code yang ditentukan.
// Digunakan oleh semua handler — menggantikan writeBedsJSON dan writeSKJSON.
func writeJSON(c *gin.Context, status int, data interface{}) {
	c.JSON(status, data)
}

// setCORSHeader menetapkan header CORS sesuai origin yang dikonfigurasi.
// Dipanggil di awal setiap handler agar header ada sebelum WriteHeader.
func setCORSHeader(c *gin.Context, origin string) {
	if origin == "" {
		origin = "*"
	}
	c.Header("Access-Control-Allow-Origin", origin)
}
