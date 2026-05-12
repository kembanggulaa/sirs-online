package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

	"github.com/gin-gonic/gin"

	"sirs-online/config"
	"sirs-online/internal/logger"
	"sirs-online/internal/worker"
)

// ProxyHandler menangani semua endpoint proxy ke API Kemenkes.
// Dipisahkan dari main.go agar testable dan modular.
type ProxyHandler struct {
	cfg *config.Config
}

// NewProxyHandler membuat ProxyHandler baru.
func NewProxyHandler(cfg *config.Config) *ProxyHandler {
	return &ProxyHandler{cfg: cfg}
}

// RegisterRoutes mendaftarkan semua route proxy ke Gin Engine.
func (h *ProxyHandler) RegisterRoutes(r *gin.Engine) {
	// Tab 2: GET referensi TT dari Kemenkes
	r.GET("/api/proxy/referensi", h.makeProxyHandler("GET",
		h.cfg.API.URL+"/Referensi/tempat_tidur"))

	// Tab 3: GET data Fasyankes yang sudah diinputkan RS
	r.GET("/api/proxy/fasyankes", h.makeProxyHandler("GET",
		h.cfg.API.URL+"/Fasyankes"))

	// Tab 4: POST tempat tidur baru
	r.POST("/api/kemenkes/tempat-tidur", h.makeForwardHandler("POST",
		h.cfg.API.URL+"/Fasyankes"))

	// Tab 4: PUT tempat tidur (update manual)
	r.PUT("/api/kemenkes/tempat-tidur/:id_tt", h.makeForwardHandler("PUT",
		h.cfg.API.URL+"/Fasyankes"))

	// Dashboard Eksekutif
	r.GET("/api/beds/executive", h.makeProxyHandler("GET",
		h.cfg.API.ExecutiveURL))
}

// makeProxyHandler membuat handler GET read-only ke Kemenkes (untuk Tab 2 & 3).
// Menggunakan shared client dengan TLS skip verify dan logging diagnostik.
func (h *ProxyHandler) makeProxyHandler(method, url string) gin.HandlerFunc {
	return func(c *gin.Context) {
		client := worker.NewKemenkesClient(h.cfg.Operational.TLSSkipVerify)
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		logger.Info("[PROXY] %s %s", method, url)

		resp, err := client.R().
			SetHeader("X-rs-id", h.cfg.API.RsID).
			SetHeader("X-pass", h.cfg.API.Pass).
			SetHeader("X-Timestamp", timestamp).
			Execute(method, url)

		if err != nil {
			logger.Error("[PROXY] Gagal %s %s: %v", method, url, err)
			writeJSON(c, http.StatusBadGateway, map[string]string{
				"error": "Gagal menghubungi API Kemenkes: " + err.Error(),
			})
			return
		}

		logger.Info("[PROXY] %s %s → status %d (%d bytes)",
			method, url, resp.StatusCode(), len(resp.Body()))

		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Writer.WriteHeader(resp.StatusCode())
		_, _ = c.Writer.Write(resp.Body())
	}
}

// makeForwardHandler meneruskan request POST/PUT dari dashboard ke Kemenkes.
func (h *ProxyHandler) makeForwardHandler(method, url string) gin.HandlerFunc {
	return func(c *gin.Context) {
		maxBytes := h.cfg.Security.MaxBodyBytes
		if maxBytes <= 0 {
			maxBytes = 1 << 20 // 1 MB
		}
		c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
		defer c.Request.Body.Close()

		var body map[string]string
		if err := json.NewDecoder(c.Request.Body).Decode(&body); err != nil {
			writeJSON(c, http.StatusBadRequest, map[string]string{
				"error": "Body tidak valid: " + err.Error(),
			})
			return
		}

		client := worker.NewKemenkesClient(h.cfg.Operational.TLSSkipVerify)
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		logger.Info("[PROXY] %s %s", method, url)

		resp, err := client.R().
			SetHeader("X-rs-id", h.cfg.API.RsID).
			SetHeader("X-pass", h.cfg.API.Pass).
			SetHeader("X-Timestamp", timestamp).
			SetHeader("Content-Type", "application/json").
			SetBody(body).
			Execute(method, url)

		if err != nil {
			logger.Error("[PROXY] Gagal %s %s: %v", method, url, err)
			writeJSON(c, http.StatusBadGateway, map[string]string{
				"error": "Gagal menghubungi API Kemenkes: " + err.Error(),
			})
			return
		}

		logger.Info("[PROXY] %s %s → status %d (%d bytes)",
			method, url, resp.StatusCode(), len(resp.Body()))

		c.Header("Content-Type", "application/json; charset=utf-8")
		c.Writer.WriteHeader(resp.StatusCode())
		_, _ = c.Writer.Write(resp.Body())
	}
}