package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"sirs-online/config"
	"sirs-online/internal/repository"
)

type SKHandler struct {
	repo repository.SKRepositoryInterface
	cfg  *config.Config
}

func NewSKHandler(repo repository.SKRepositoryInterface, cfg *config.Config) *SKHandler {
	return &SKHandler{repo: repo, cfg: cfg}
}

func (h *SKHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/sk/list", h.handleGetSKList)
	r.GET("/api/sk/detail", h.handleGetSKDetail)
	r.POST("/api/sk/preview", h.handlePostSKPreview)
	r.POST("/api/sk/import", h.handlePostSKImport)
}

func (h *SKHandler) maxBodyBytes() int64 {
	if h.cfg != nil && h.cfg.Security.MaxBodyBytes > 0 {
		return h.cfg.Security.MaxBodyBytes
	}
	return 1 << 20 // 1 MB default
}

func (h *SKHandler) handleGetSKList(c *gin.Context) {
	list, err := h.repo.GetSKList(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan daftar SK: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, list)
}

func (h *SKHandler) handleGetSKDetail(c *gin.Context) {
	skNo := c.Query("sk_no")
	if skNo == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "parameter sk_no dibutuhkan"})
		return
	}

	detail, err := h.repo.GetSKDetail(c.Request.Context(), skNo)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan detail SK: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, detail)
}

func (h *SKHandler) handlePostSKPreview(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBodyBytes())
	defer c.Request.Body.Close()

	var req repository.SKImportRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	if req.SKNo == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "sk_no tidak boleh kosong"})
		return
	}
	if req.TglBerlaku == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "tgl_berlaku tidak boleh kosong"})
		return
	}
	if len(req.Rows) == 0 {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "rows tidak boleh kosong"})
		return
	}

	response := map[string]interface{}{
		"sk_no":       req.SKNo,
		"tgl_berlaku": req.TglBerlaku,
		"total_rows":  len(req.Rows),
		"rows":        req.Rows,
	}

	c.JSON(http.StatusOK, response)
}

func (h *SKHandler) handlePostSKImport(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBodyBytes())
	defer c.Request.Body.Close()

	var req repository.SKImportRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	if req.SKNo == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "sk_no tidak boleh kosong"})
		return
	}
	if req.TglBerlaku == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "tgl_berlaku tidak boleh kosong"})
		return
	}
	if len(req.Rows) == 0 {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "rows tidak boleh kosong"})
		return
	}

	inserted, err := h.repo.BulkInsertSKBed(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal import SK: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, map[string]interface{}{
		"inserted": inserted,
		"sk_no":    req.SKNo,
		"message":  "Import berhasil",
	})
}