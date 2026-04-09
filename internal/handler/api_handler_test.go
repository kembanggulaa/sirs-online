package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
)

// ─── Helper untuk memeriksa header umum ──────────────────────────────────────

func assertCommonHeaders(t *testing.T, w *httptest.ResponseRecorder) {
	t.Helper()
	ct := w.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want prefix %q", ct, "application/json")
	}
	cors := w.Header().Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("Access-Control-Allow-Origin: got %q, want %q", cors, "*")
	}
}

// ─── writeJSON ────────────────────────────────────────────────────────────────

func TestWriteJSON_StatusAndBody(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeJSON(w, http.StatusTeapot, map[string]string{"key": "value"})

	if w.Code != http.StatusTeapot {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusTeapot)
	}
	assertCommonHeaders(t, w)

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("body[key]: got %q, want %q", got["key"], "value")
	}
}

// ─── writeBedsJSON / writeBedsError ──────────────────────────────────────────

func TestWriteBedsJSON_StatusAndBody(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeBedsJSON(w, http.StatusCreated, map[string]int{"count": 3})

	if w.Code != http.StatusCreated {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusCreated)
	}
	assertCommonHeaders(t, w)
}

func TestWriteBedsError_ReturnsBadRequest(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeBedsError(w, http.StatusBadRequest, "parameter wajib diisi")

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["error"] != "parameter wajib diisi" {
		t.Errorf("error message: got %q, want %q", got["error"], "parameter wajib diisi")
	}
}

// ─── writeSKJSON / writeSKError ──────────────────────────────────────────────

func TestWriteSKJSON_StatusAndBody(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeSKJSON(w, http.StatusOK, map[string]string{"sk_no": "SK-001"})

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertCommonHeaders(t, w)

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["sk_no"] != "SK-001" {
		t.Errorf("sk_no: got %q, want %q", got["sk_no"], "SK-001")
	}
}

func TestWriteSKError_ReturnsInternalServerError(t *testing.T) {
	t.Parallel()

	w := httptest.NewRecorder()
	writeSKError(w, http.StatusInternalServerError, "koneksi DB gagal")

	if w.Code != http.StatusInternalServerError {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusInternalServerError)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["error"] != "koneksi DB gagal" {
		t.Errorf("error message: got %q, want %q", got["error"], "koneksi DB gagal")
	}
}

// ─── BedsHandler: Validasi Input ─────────────────────────────────────────────

// handleGetKamar harus mengembalikan 400 jika parameter class_room_id tidak diisi
func TestBedsHandler_GetKamar_MissingParam(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil} // repo nil: OK karena validasi sebelum DB call
	req := httptest.NewRequest("GET", "/api/beds/kamar", nil)
	w := httptest.NewRecorder()

	h.handleGetKamar(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
	var got map[string]string
	_ = json.NewDecoder(w.Body).Decode(&got)
	if got["error"] == "" {
		t.Error("expected error field in response, got empty")
	}
}

// handleGetBedsByRoom harus mengembalikan 400 jika parameter class_room_id kosong
func TestBedsHandler_GetBedsByRoom_MissingParam(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil}
	req := httptest.NewRequest("GET", "/api/beds/by-room", nil)
	w := httptest.NewRecorder()

	h.handleGetBedsByRoom(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostUpsertBeds harus mengembalikan 400 untuk JSON tidak valid
func TestBedsHandler_Upsert_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil}
	req := httptest.NewRequest("POST", "/api/beds/upsert", strings.NewReader("bukan-json"))
	w := httptest.NewRecorder()

	h.handlePostUpsertBeds(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostUpsertBeds harus mengembalikan 400 jika class_room_id kosong
func TestBedsHandler_Upsert_MissingClassRoomID(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil}
	body := `{"class_room_id":"","rows":[{"kamar_id":"K1"}]}`
	req := httptest.NewRequest("POST", "/api/beds/upsert", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.handlePostUpsertBeds(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostUpsertBeds harus mengembalikan 400 jika rows kosong
func TestBedsHandler_Upsert_EmptyRows(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil}
	body := `{"class_room_id":"R-VIP","rows":[]}`
	req := httptest.NewRequest("POST", "/api/beds/upsert", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.handlePostUpsertBeds(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── SKHandler: Validasi Input ────────────────────────────────────────────────

// handleGetSKDetail harus mengembalikan 400 jika sk_no tidak diisi
func TestSKHandler_GetDetail_MissingParam(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil}
	req := httptest.NewRequest("GET", "/api/sk/detail", nil)
	w := httptest.NewRecorder()

	h.handleGetSKDetail(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview harus mengembalikan 400 untuk JSON tidak valid
func TestSKHandler_Preview_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil}
	req := httptest.NewRequest("POST", "/api/sk/preview", strings.NewReader("<<<json rusak>>>"))
	w := httptest.NewRecorder()

	h.handlePostSKPreview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview harus mengembalikan 400 jika sk_no kosong
func TestSKHandler_Preview_MissingSKNo(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil}
	body := `{"sk_no":"","rows":[{"id_tt":"TT001"}]}`
	req := httptest.NewRequest("POST", "/api/sk/preview", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.handlePostSKPreview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview harus mengembalikan 400 jika rows kosong
func TestSKHandler_Preview_EmptyRows(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil}
	body := `{"sk_no":"SK/001/2024","rows":[]}`
	req := httptest.NewRequest("POST", "/api/sk/preview", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.handlePostSKPreview(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview — sukses mengembalikan preview data
func TestSKHandler_Preview_Success(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil}
	body := `{"sk_no":"SK/001/2024","tgl_berlaku":"2024-01-01","rows":[{"id_tt":"TT001","nama_tt":"VIP A"}]}`
	req := httptest.NewRequest("POST", "/api/sk/preview", strings.NewReader(body))
	w := httptest.NewRecorder()

	h.handlePostSKPreview(w, req)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var got map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["sk_no"] != "SK/001/2024" {
		t.Errorf("sk_no: got %v, want %q", got["sk_no"], "SK/001/2024")
	}
	if got["total_rows"].(float64) != 1 {
		t.Errorf("total_rows: got %v, want 1", got["total_rows"])
	}
}

// handlePostSKImport harus mengembalikan 400 untuk JSON tidak valid
func TestSKHandler_Import_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil}
	req := httptest.NewRequest("POST", "/api/sk/import", strings.NewReader("<<<bukan json>>>"))
	w := httptest.NewRecorder()

	h.handlePostSKImport(w, req)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}
