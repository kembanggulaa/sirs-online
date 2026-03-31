package handler

import (
	"encoding/json"
	"net/http"

	"sirs-online/internal/repository"
)

type BedsHandler struct {
	repo *repository.BedsRepository
}

func NewBedsHandler(repo *repository.BedsRepository) *BedsHandler {
	return &BedsHandler{repo: repo}
}

func (h *BedsHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/beds/rooms", h.handleGetRooms)
	mux.HandleFunc("GET /api/beds/kamar", h.handleGetKamar)
	mux.HandleFunc("GET /api/beds/by-room", h.handleGetBedsByRoom)
	mux.HandleFunc("POST /api/beds/upsert", h.handlePostUpsertBeds)
}

func (h *BedsHandler) handleGetRooms(w http.ResponseWriter, r *http.Request) {
	rooms, err := h.repo.GetDistinctClassRooms(r.Context())
	if err != nil {
		writeBedsError(w, http.StatusInternalServerError, "Gagal mendapatkan class_room_id: "+err.Error())
		return
	}
	writeBedsJSON(w, http.StatusOK, rooms)
}

func (h *BedsHandler) handleGetKamar(w http.ResponseWriter, r *http.Request) {
	classRoomID := r.URL.Query().Get("class_room_id")
	if classRoomID == "" {
		writeBedsError(w, http.StatusBadRequest, "parameter class_room_id wajib diisi")
		return
	}

	kamarList, err := h.repo.GetKamarByClassRoom(r.Context(), classRoomID)
	if err != nil {
		writeBedsError(w, http.StatusInternalServerError, "Gagal mendapatkan kamar: "+err.Error())
		return
	}
	writeBedsJSON(w, http.StatusOK, kamarList)
}

func (h *BedsHandler) handleGetBedsByRoom(w http.ResponseWriter, r *http.Request) {
	classRoomID := r.URL.Query().Get("class_room_id")

	if classRoomID == "" {
		writeBedsError(w, http.StatusBadRequest, "parameter class_room_id wajib diisi")
		return
	}

	result, err := h.repo.GetBedsByRoom(r.Context(), classRoomID)
	if err != nil {
		writeBedsError(w, http.StatusInternalServerError, "Gagal mendapatkan data beds: "+err.Error())
		return
	}

	writeBedsJSON(w, http.StatusOK, result)
}

func (h *BedsHandler) handlePostUpsertBeds(w http.ResponseWriter, r *http.Request) {
	var req repository.BedsUpsertRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeBedsError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.ClassRoomID == "" {
		writeBedsError(w, http.StatusBadRequest, "class_room_id tidak boleh kosong")
		return
	}
	if len(req.Rows) == 0 {
		writeBedsError(w, http.StatusBadRequest, "rows tidak boleh kosong")
		return
	}

	res, err := h.repo.UpsertBeds(r.Context(), req)
	if err != nil {
		writeBedsError(w, http.StatusInternalServerError, "Gagal upsert beds: "+err.Error())
		return
	}

	writeBedsJSON(w, http.StatusOK, res)
}

func writeBedsError(w http.ResponseWriter, status int, msg string) {
	writeBedsJSON(w, status, map[string]string{"error": msg})
}

func writeBedsJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
