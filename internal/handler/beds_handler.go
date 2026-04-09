package handler

import (
	"encoding/json"
	"net/http"

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

func (h *BedsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/beds/rooms", h.handleGetRooms)
	mux.HandleFunc("GET /api/beds/kamar", h.handleGetKamar)
	mux.HandleFunc("GET /api/beds/by-room", h.handleGetBedsByRoom)
	mux.HandleFunc("POST /api/beds/upsert", h.handlePostUpsertBeds)
}

func (h *BedsHandler) corsOrigin() string {
	if h.cfg != nil {
		return h.cfg.DashboardOrigin
	}
	return "*"
}

func (h *BedsHandler) maxBodyBytes() int64 {
	if h.cfg != nil && h.cfg.MaxBodyBytes > 0 {
		return h.cfg.MaxBodyBytes
	}
	return 1 << 20 // 1 MB default
}

func (h *BedsHandler) handleGetRooms(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	rooms, err := h.repo.GetDistinctClassRooms(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan class_room_id: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, rooms)
}

func (h *BedsHandler) handleGetKamar(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	classRoomID := r.URL.Query().Get("class_room_id")
	if classRoomID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter class_room_id wajib diisi"})
		return
	}

	kamarList, err := h.repo.GetKamarByClassRoom(r.Context(), classRoomID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan kamar: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, kamarList)
}

func (h *BedsHandler) handleGetBedsByRoom(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	classRoomID := r.URL.Query().Get("class_room_id")

	if classRoomID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter class_room_id wajib diisi"})
		return
	}

	result, err := h.repo.GetBedsByRoom(r.Context(), classRoomID)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan data beds: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, result)
}

func (h *BedsHandler) handlePostUpsertBeds(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes())
	defer r.Body.Close()

	var req repository.BedsUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	if req.ClassRoomID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "class_room_id tidak boleh kosong"})
		return
	}
	if len(req.Rows) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rows tidak boleh kosong"})
		return
	}

	res, err := h.repo.UpsertBeds(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal upsert beds: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, res)
}
