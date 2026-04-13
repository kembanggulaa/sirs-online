package handler

import (
	"encoding/json"
	"net/http"

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

func (h *SKHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/sk/list", h.handleGetSKList)
	mux.HandleFunc("GET /api/sk/detail", h.handleGetSKDetail)
	mux.HandleFunc("POST /api/sk/preview", h.handlePostSKPreview)
	mux.HandleFunc("POST /api/sk/import", h.handlePostSKImport)
}

func (h *SKHandler) corsOrigin() string {
	if h.cfg != nil {
		return h.cfg.Security.DashboardOrigin
	}
	return "*"
}

func (h *SKHandler) maxBodyBytes() int64 {
	if h.cfg != nil && h.cfg.Security.MaxBodyBytes > 0 {
		return h.cfg.Security.MaxBodyBytes
	}
	return 1 << 20 // 1 MB default
}

func (h *SKHandler) handleGetSKList(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	list, err := h.repo.GetSKList(r.Context())
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan daftar SK: " + err.Error()})
		return
	}
	writeJSON(w, http.StatusOK, list)
}

func (h *SKHandler) handleGetSKDetail(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	skNo := r.URL.Query().Get("sk_no")
	if skNo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "parameter sk_no dibutuhkan"})
		return
	}

	detail, err := h.repo.GetSKDetail(r.Context(), skNo)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal mendapatkan detail SK: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, detail)
}

func (h *SKHandler) handlePostSKPreview(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes())
	defer r.Body.Close()

	var req repository.SKImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	if req.SKNo == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "sk_no tidak boleh kosong"})
		return
	}
	if len(req.Rows) == 0 {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "rows tidak boleh kosong"})
		return
	}

	response := map[string]interface{}{
		"sk_no":       req.SKNo,
		"tgl_berlaku": req.TglBerlaku,
		"total_rows":  len(req.Rows),
		"rows":        req.Rows,
	}

	writeJSON(w, http.StatusOK, response)
}

func (h *SKHandler) handlePostSKImport(w http.ResponseWriter, r *http.Request) {
	setCORSHeader(w, h.corsOrigin())
	r.Body = http.MaxBytesReader(w, r.Body, h.maxBodyBytes())
	defer r.Body.Close()

	var req repository.SKImportRequest
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "Invalid JSON payload: " + err.Error()})
		return
	}

	inserted, err := h.repo.BulkInsertSKBed(r.Context(), req)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{"error": "Gagal import SK: " + err.Error()})
		return
	}

	writeJSON(w, http.StatusOK, map[string]interface{}{
		"inserted": inserted,
		"sk_no":    req.SKNo,
		"message":  "Import berhasil",
	})
}
