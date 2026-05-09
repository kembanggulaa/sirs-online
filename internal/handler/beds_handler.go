package handler

import (
	"encoding/json"
	"net/http"

	"github.com/gin-gonic/gin"
	"sirs-online/config"
	"sirs-online/internal/repository"
)

type BedsHandler struct {
	repo repository.BedsRepositoryInterface
	cfg  *config.Config
}

func NewBedsHandler(repo repository.BedsRepositoryInterface, cfg *config.Config) *BedsHandler {
	return &BedsHandler{repo: repo, cfg: cfg}
}

func (h *BedsHandler) RegisterRoutes(r *gin.Engine) {
	r.GET("/api/beds/rooms", h.handleGetRooms)
	r.GET("/api/beds/kamar", h.handleGetKamar)
	r.GET("/api/beds/by-room", h.handleGetBedsByRoom)
	r.POST("/api/beds/upsert", h.handlePostUpsertBeds)
}

func (h *BedsHandler) corsOrigin() string {
	if h.cfg != nil {
		return h.cfg.Security.DashboardOrigin
	}
	return "*"
}

func (h *BedsHandler) maxBodyBytes() int64 {
	if h.cfg != nil && h.cfg.Security.MaxBodyBytes > 0 {
		return h.cfg.Security.MaxBodyBytes
	}
	return 1 << 20 // 1 MB default
}

func (h *BedsHandler) handleGetRooms(c *gin.Context) {
	rooms, err := h.repo.GetDistinctClassRooms(c.Request.Context())
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan class_room_id: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, rooms)
}

func (h *BedsHandler) handleGetKamar(c *gin.Context) {
	classRoomID := c.Query("class_room_id")
	if classRoomID == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "parameter class_room_id wajib diisi"})
		return
	}

	kamarList, err := h.repo.GetKamarByClassRoom(c.Request.Context(), classRoomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan kamar: " + err.Error()})
		return
	}
	c.JSON(http.StatusOK, kamarList)
}

func (h *BedsHandler) handleGetBedsByRoom(c *gin.Context) {
	classRoomID := c.Query("class_room_id")

	if classRoomID == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "parameter class_room_id wajib diisi"})
		return
	}

	result, err := h.repo.GetBedsByRoom(c.Request.Context(), classRoomID)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan data beds: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, result)
}

func (h *BedsHandler) handlePostUpsertBeds(c *gin.Context) {
	c.Request.Body = http.MaxBytesReader(c.Writer, c.Request.Body, h.maxBodyBytes())
	defer c.Request.Body.Close()

	var req repository.BedsUpsertRequest
	if err := json.NewDecoder(c.Request.Body).Decode(&req); err != nil {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	if req.ClassRoomID == "" {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "class_room_id tidak boleh kosong"})
		return
	}
	if len(req.Rows) == 0 {
		c.JSON(http.StatusBadRequest, map[string]string{"error": "rows tidak boleh kosong"})
		return
	}

	res, err := h.repo.UpsertBeds(c.Request.Context(), req)
	if err != nil {
		c.JSON(http.StatusInternalServerError, map[string]string{"error": "Gagal upsert beds: " + err.Error()})
		return
	}

	c.JSON(http.StatusOK, res)
}
