package handler

import (
	"encoding/json"
	"net/http"
)

// writeJSON menulis response JSON dengan status code yang ditentukan.
// Digunakan oleh semua handler — menggantikan writeBedsJSON dan writeSKJSON.
func writeJSON(w http.ResponseWriter, status int, data interface{}) {
	w.Header().Set("Content-Type", "application/json; charset=utf-8")
	w.WriteHeader(status)
	_ = json.NewEncoder(w).Encode(data)
}

// setCORSHeader menetapkan header CORS sesuai origin yang dikonfigurasi.
// Dipanggil di awal setiap handler agar header ada sebelum WriteHeader.
func setCORSHeader(w http.ResponseWriter, origin string) {
	if origin == "" {
		origin = "*"
	}
	w.Header().Set("Access-Control-Allow-Origin", origin)
}
