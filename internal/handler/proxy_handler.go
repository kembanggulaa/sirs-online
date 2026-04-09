package handler

import (
	"encoding/json"
	"net/http"
	"strconv"
	"time"

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

// RegisterRoutes mendaftarkan semua route proxy ke ServeMux.
func (h *ProxyHandler) RegisterRoutes(mux *http.ServeMux) {
	// Tab 2: GET referensi TT dari Kemenkes
	mux.HandleFunc("GET /api/proxy/referensi", h.makeProxyHandler("GET",
		h.cfg.APIURL+"/Referensi/tempat_tidur"))

	// Tab 3: GET data Fasyankes yang sudah diinputkan RS
	mux.HandleFunc("GET /api/proxy/fasyankes", h.makeProxyHandler("GET",
		h.cfg.APIURL+"/Fasyankes"))

	// Tab 4: POST tempat tidur baru
	mux.HandleFunc("POST /api/kemenkes/tempat-tidur", h.makeForwardHandler("POST",
		h.cfg.APIURL+"/Fasyankes"))

	// Tab 4: PUT tempat tidur (update manual)
	mux.HandleFunc("PUT /api/kemenkes/tempat-tidur/{id_tt}", h.makeForwardHandler("PUT",
		h.cfg.APIURL+"/Fasyankes"))

	// Dashboard Eksekutif
	mux.HandleFunc("GET /api/beds/executive", h.makeProxyHandler("GET",
		h.cfg.ExecutiveAPIURL))
}

// makeProxyHandler membuat handler GET read-only ke Kemenkes (untuk Tab 2 & 3).
// Menggunakan shared client dengan TLS skip verify dan logging diagnostik.
func (h *ProxyHandler) makeProxyHandler(method, url string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeader(w, h.cfg.DashboardOrigin)
		client := worker.NewKemenkesClient(h.cfg.TLSSkipVerify)
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		logger.Info("[PROXY] %s %s", method, url)

		resp, err := client.R().
			SetHeader("X-rs-id", h.cfg.APIRsID).
			SetHeader("X-pass", h.cfg.APIPass).
			SetHeader("X-Timestamp", timestamp).
			Execute(method, url)

		if err != nil {
			logger.Error("[PROXY] Gagal %s %s: %v", method, url, err)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "Gagal menghubungi API Kemenkes: " + err.Error(),
			})
			return
		}

		logger.Info("[PROXY] %s %s → status %d (%d bytes)",
			method, url, resp.StatusCode(), len(resp.Body()))

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.StatusCode())
		_, _ = w.Write(resp.Body())
	}
}

// makeForwardHandler meneruskan request POST/PUT dari dashboard ke Kemenkes.
func (h *ProxyHandler) makeForwardHandler(method, url string) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		setCORSHeader(w, h.cfg.DashboardOrigin)

		maxBytes := h.cfg.MaxBodyBytes
		if maxBytes <= 0 {
			maxBytes = 1 << 20 // 1 MB
		}
		r.Body = http.MaxBytesReader(w, r.Body, maxBytes)
		defer r.Body.Close()

		var body map[string]string
		if err := json.NewDecoder(r.Body).Decode(&body); err != nil {
			writeJSON(w, http.StatusBadRequest, map[string]string{
				"error": "Body tidak valid: " + err.Error(),
			})
			return
		}

		client := worker.NewKemenkesClient(h.cfg.TLSSkipVerify)
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		logger.Info("[PROXY] %s %s", method, url)

		resp, err := client.R().
			SetHeader("X-rs-id", h.cfg.APIRsID).
			SetHeader("X-pass", h.cfg.APIPass).
			SetHeader("X-Timestamp", timestamp).
			SetHeader("Content-Type", "application/json").
			SetBody(body).
			Execute(method, url)

		if err != nil {
			logger.Error("[PROXY] Gagal %s %s: %v", method, url, err)
			writeJSON(w, http.StatusBadGateway, map[string]string{
				"error": "Gagal menghubungi API Kemenkes: " + err.Error(),
			})
			return
		}

		logger.Info("[PROXY] %s %s → status %d (%d bytes)",
			method, url, resp.StatusCode(), len(resp.Body()))

		w.Header().Set("Content-Type", "application/json; charset=utf-8")
		w.WriteHeader(resp.StatusCode())
		_, _ = w.Write(resp.Body())
	}
}
