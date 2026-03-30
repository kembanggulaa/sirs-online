package handler

import (
	"encoding/json"
	"net/http"

	"sirs-online/internal/repository"
)

type SKHandler struct {
	repo *repository.SKRepository
}

func NewSKHandler(repo *repository.SKRepository) *SKHandler {
	return &SKHandler{repo: repo}
}

func (h *SKHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sk/list", h.handleGetSKList)
	mux.HandleFunc("GET /api/sk/detail", h.handleGetSKDetail)
	mux.HandleFunc("POST /api/sk/preview", h.handlePostSKPreview)
	mux.HandleFunc("POST /api/sk/import", h.handlePostSKImport)
}

func (h *SKHandler) handleGetSKList(w http.ResponseWriter, r *http.Request) {
	list, err := h.repo.GetSKList(r.Context())
	if err != nil {
		writeSKError(w, http.StatusInternalServerError, "Gagal mendapatkan daftar SK: "+err.Error())
		return
	}
	writeSKJSON(w, http.StatusOK, list)
}

func (h *SKHandler) handleGetSKDetail(w http.ResponseWriter, r *http.Request) {
	skNo := r.URL.Query().Get("sk_no")
	if skNo == "" {
		writeSKError(w, http.StatusBadRequest, "parameter sk_no dibutuhkan")
		return
	}

	detail, err := h.repo.GetSKDetail(r.Context(), skNo)
	if err != nil {
		writeSKError(w, http.StatusInternalServerError, "Gagal mendapatkan detail SK: "+err.Error())
		return
	}

	writeSKJSON(w, http.StatusOK, detail)
}

func (h *SKHandler) handlePostSKPreview(w http.ResponseWriter, r *http.Request) {
	var req repository.SKImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSKError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	if req.SKNo == "" {
		writeSKError(w, http.StatusBadRequest, "sk_no tidak boleh kosong")
		return
	}
	if len(req.Rows) == 0 {
		writeSKError(w, http.StatusBadRequest, "rows tidak boleh kosong")
		return
	}

	response := map[string]interface{}{
		"sk_no":       req.SKNo,
		"tgl_berlaku": req.TglBerlaku,
		"total_rows":  len(req.Rows),
		"rows":        req.Rows,
	}

	writeSKJSON(w, http.StatusOK, response)
}

func (h *SKHandler) handlePostSKImport(w http.ResponseWriter, r *http.Request) {
	var req repository.SKImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeSKError(w, http.StatusBadRequest, "Invalid JSON payload: "+err.Error())
		return
	}
	defer r.Body.Close()

	inserted, err := h.repo.BulkInsertSKBed(r.Context(), req)
	if err != nil {
		writeSKError(w, http.StatusInternalServerError, "Gagal import SK: "+err.Error())
		return
	}

	writeSKJSON(w, http.StatusOK, map[string]interface{}{
		"inserted": inserted,
		"sk_no":    req.SKNo,
		"message":  "Import berhasil",
	})
}

func writeSKError(w http.ResponseWriter, status int, msg string) {
	writeSKJSON(w, status, map[string]string{"error": msg})
}

func writeSKJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
