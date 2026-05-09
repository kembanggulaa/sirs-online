package handler

import (
	"net/http"

	"github.com/gin-gonic/gin"

	"sirs-online/config"
	"sirs-online/internal/logger"
	"sirs-online/internal/worker"
)

// APIHandler menggabungkan semua endpoint REST API internal
type APIHandler struct {
	cfg        *config.Config
	dispatcher *worker.Dispatcher
}

// New membuat APIHandler baru
func New(cfg *config.Config, dispatcher *worker.Dispatcher) *APIHandler {
	return &APIHandler{cfg: cfg, dispatcher: dispatcher}
}

// RegisterRoutes mendaftarkan semua route ke Gin Engine
func (h *APIHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/beds", h.handleGetBeds)
	r.GET("/api/logs", h.handleGetLogs)
	r.POST("/api/sync", h.handlePostSync)
	r.GET("/api/worker/status", h.handleWorkerStatus)
	r.GET("/api/sk-active", h.handleSKActive)
	r.GET("/api/healthz", h.handleHealthz)
	r.Static("/", "web/static")
}

// handleGetBeds — GET /api/beds
// Mengembalikan data ketersediaan bed in-memory sebagai JSON
func (h *APIHandler) handleGetBeds(c *gin.Context) {
	beds := worker.GetBeds()
	c.JSON(http.StatusOK, beds)
}

// handleGetLogs — GET /api/logs
// Mengembalikan 200 baris terakhir dari file log
func (h *APIHandler) handleGetLogs(c *gin.Context) {
	lines, err := logger.ReadLast(h.cfg.Operational.LogFile, 200)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{
			"error": "Gagal membaca file log: " + err.Error(),
		})
		return
	}
	c.JSON(http.StatusOK, map[string]interface{}{
		"lines": lines,
	})
}

// handlePostSync — POST /api/sync
// Memicu sinkronisasi manual dari dashboard
func (h *APIHandler) handlePostSync(c *gin.Context) {
	maxBytes := h.cfg.Security.MaxBodyBytes
	if maxBytes <= 0 {
		maxBytes = 1 << 20 // 1 MB default
	}
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, maxBytes)
	defer c.Request.Body.Close()

	triggered := h.dispatcher.TriggerManual()
	if triggered {
		c.JSON(http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "Sinkronisasi manual dimulai",
		})
	} else {
		c.JSON(http.StatusConflict, map[string]string{
			"status":  "busy",
			"message": "Worker sedang berjalan, sync di-skip",
		})
	}
}

// handleWorkerStatus — GET /api/worker/status
// Mengembalikan status worker: Running atau Idle
func (h *APIHandler) handleWorkerStatus(c *gin.Context) {
	status := "Idle"
	if h.dispatcher.IsRunning() {
		status = "Running"
	}
	c.JSON(http.StatusOK, map[string]string{
		"status": status,
	})
}

// handleSKActive — GET /api/sk-active
// Mengembalikan sk_no aktif yang terdeteksi dari DB
func (h *APIHandler) handleSKActive(c *gin.Context) {
	skNo := worker.GetActiveSKNoCurrent()
	c.JSON(http.StatusOK, map[string]string{
		"sk_no": skNo,
	})
}

// handleHealthz — GET /api/healthz
// Health check endpoint untuk monitoring (Phase 5 task pulled forward)
func (h *APIHandler) handleHealthz(c *gin.Context) {
	c.JSON(http.StatusOK, map[string]string{
		"status": "ok",
	})
}