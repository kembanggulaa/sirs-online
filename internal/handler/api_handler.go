package handler

import (
	"encoding/json"
	"net/http"

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

// RegisterRoutes mendaftarkan semua route ke ServeMux
func (h *APIHandler) RegisterRoutes(mux *http.ServeMux) {
	mux.HandleFunc("GET /api/beds", h.handleGetBeds)
	mux.HandleFunc("GET /api/logs", h.handleGetLogs)
	mux.HandleFunc("POST /api/sync", h.handlePostSync)
	mux.HandleFunc("GET /api/worker/status", h.handleWorkerStatus)
	mux.HandleFunc("GET /api/sk-active", h.handleSKActive)

	// Static files (dashboard)
	mux.Handle("/", http.FileServer(http.Dir("web/static")))
}

// handleGetBeds — GET /api/beds
// Mengembalikan data ketersediaan bed in-memory sebagai JSON
func (h *APIHandler) handleGetBeds(w http.ResponseWriter, r *http.Request) {
	beds := worker.GetBeds()
	writeJSON(w, http.StatusOK, beds)
}

// handleGetLogs — GET /api/logs
// Mengembalikan 200 baris terakhir dari file log
func (h *APIHandler) handleGetLogs(w http.ResponseWriter, r *http.Request) {
	lines, err := logger.ReadLast(h.cfg.LogFile, 200)
	if err != nil {
		writeJSON(w, http.StatusInternalServerError, map[string]string{
			"error": "Gagal membaca file log: " + err.Error(),
		})
		return
	}
	writeJSON(w, http.StatusOK, map[string]interface{}{
		"lines": lines,
	})
}

// handlePostSync — POST /api/sync
// Memicu sinkronisasi manual dari dashboard
func (h *APIHandler) handlePostSync(w http.ResponseWriter, r *http.Request) {
	triggered := h.dispatcher.TriggerManual()
	if triggered {
		writeJSON(w, http.StatusOK, map[string]string{
			"status":  "ok",
			"message": "Sinkronisasi manual dimulai",
		})
	} else {
		writeJSON(w, http.StatusConflict, map[string]string{
			"status":  "busy",
			"message": "Worker sedang berjalan, sync di-skip",
		})
	}
}

// handleWorkerStatus — GET /api/worker/status
// Mengembalikan status worker: Running atau Idle
func (h *APIHandler) handleWorkerStatus(w http.ResponseWriter, r *http.Request) {
	status := "Idle"
	if h.dispatcher.IsRunning() {
		status = "Running"
	}
	writeJSON(w, http.StatusOK, map[string]string{
		"status": status,
	})
}

// handleSKActive — GET /api/sk-active
// Mengembalikan sk_no aktif yang terdeteksi dari DB
func (h *APIHandler) handleSKActive(w http.ResponseWriter, r *http.Request) {
	skNo := worker.GetActiveSKNoCurrent()
	writeJSON(w, http.StatusOK, map[string]string{
		"sk_no": skNo,
	})
}

// writeJSON menulis response JSON dengan status code yang ditentukan
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.Header().Set("Access-Control-Allow-Origin", "*")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}
