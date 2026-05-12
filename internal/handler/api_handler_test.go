package handler

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"

	"github.com/gin-gonic/gin"
	"sirs-online/config"
	"sirs-online/internal/repository"
	"sirs-online/internal/worker"
)

// createTestContext is a helper to create *gin.Context for testing
func createTestContext(method, path string, body *strings.Reader) (*gin.Context, *httptest.ResponseRecorder) {
	gin.SetMode(gin.TestMode)
	w := httptest.NewRecorder()
	c, _ := gin.CreateTestContext(w)
	if body != nil {
		c.Request = httptest.NewRequest(method, path, body)
	} else {
		c.Request = httptest.NewRequest(method, path, nil)
	}
	return c, w
}

// ─── Helper untuk memeriksa header Content-Type ───────────────────────────────

func assertContentTypeJSON(t *testing.T, c *gin.Context) {
	t.Helper()
	ct := c.Writer.Header().Get("Content-Type")
	if !strings.HasPrefix(ct, "application/json") {
		t.Errorf("Content-Type: got %q, want prefix %q", ct, "application/json")
	}
}

// assertCORSHeader memeriksa bahwa CORS header telah di-set (tidak kosong)
func assertCORSHeader(t *testing.T, c *gin.Context) {
	t.Helper()
	cors := c.Writer.Header().Get("Access-Control-Allow-Origin")
	if cors == "" {
		t.Error("Access-Control-Allow-Origin header tidak di-set")
	}
}

// ─── writeJSON (shared helper di response.go) ─────────────────────────────────

func TestWriteJSON_StatusAndBody(t *testing.T) {
	t.Parallel()

	c, w := createTestContext("GET", "/test", nil)
	writeJSON(c, http.StatusTeapot, map[string]string{"key": "value"})

	if w.Code != http.StatusTeapot {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusTeapot)
	}
	assertContentTypeJSON(t, c)

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["key"] != "value" {
		t.Errorf("body[key]: got %q, want %q", got["key"], "value")
	}
}

func TestWriteJSON_NilData(t *testing.T) {
	t.Parallel()

	c, w := createTestContext("GET", "/test", nil)
	writeJSON(c, http.StatusOK, nil)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertContentTypeJSON(t, c)
}

// ─── setCORSHeader ────────────────────────────────────────────────────────────

func TestSetCORSHeader_WithOrigin(t *testing.T) {
	t.Parallel()

	c, _ := createTestContext("GET", "/test", nil)
	setCORSHeader(c, "http://localhost:9271")

	cors := c.Writer.Header().Get("Access-Control-Allow-Origin")
	if cors != "http://localhost:9271" {
		t.Errorf("CORS: got %q, want %q", cors, "http://localhost:9271")
	}
}

func TestSetCORSHeader_EmptyFallsBackToWildcard(t *testing.T) {
	t.Parallel()

	c, _ := createTestContext("GET", "/test", nil)
	setCORSHeader(c, "")

	cors := c.Writer.Header().Get("Access-Control-Allow-Origin")
	if cors != "*" {
		t.Errorf("CORS fallback: got %q, want %q", cors, "*")
	}
}

// ─── BedsHandler: Validasi Input ─────────────────────────────────────────────

// handleGetKamar harus mengembalikan 400 jika parameter class_room_id tidak diisi
func TestBedsHandler_GetKamar_MissingParam(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil, cfg: nil} // repo nil: OK karena validasi sebelum DB call
	c, w := createTestContext("GET", "/api/beds/kamar", nil)

	h.handleGetKamar(c)

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

	h := &BedsHandler{repo: nil, cfg: nil}
	c, w := createTestContext("GET", "/api/beds/by-room", nil)

	h.handleGetBedsByRoom(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostUpsertBeds harus mengembalikan 400 untuk JSON tidak valid
func TestBedsHandler_Upsert_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil, cfg: nil}
	body := strings.NewReader("bukan-json")
	c, w := createTestContext("POST", "/api/beds/upsert", body)

	h.handlePostUpsertBeds(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostUpsertBeds harus mengembalikan 400 jika class_room_id kosong
func TestBedsHandler_Upsert_MissingClassRoomID(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil, cfg: nil}
	body := strings.NewReader(`{"class_room_id":"","rows":[{"kamar_id":"K1"}]}`)
	c, w := createTestContext("POST", "/api/beds/upsert", body)

	h.handlePostUpsertBeds(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostUpsertBeds harus mengembalikan 400 jika rows kosong
func TestBedsHandler_Upsert_EmptyRows(t *testing.T) {
	t.Parallel()

	h := &BedsHandler{repo: nil, cfg: nil}
	body := strings.NewReader(`{"class_room_id":"R-VIP","rows":[]}`)
	c, w := createTestContext("POST", "/api/beds/upsert", body)

	h.handlePostUpsertBeds(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── SKHandler: Validasi Input ────────────────────────────────────────────────

// handleGetSKDetail harus mengembalikan 400 jika sk_no tidak diisi
func TestSKHandler_GetDetail_MissingParam(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil, cfg: nil}
	c, w := createTestContext("GET", "/api/sk/detail", nil)

	h.handleGetSKDetail(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview harus mengembalikan 400 untuk JSON tidak valid
func TestSKHandler_Preview_InvalidJSON(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil, cfg: nil}
	body := strings.NewReader("<<<json rusak>>>")
	c, w := createTestContext("POST", "/api/sk/preview", body)

	h.handlePostSKPreview(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview harus mengembalikan 400 jika sk_no kosong
func TestSKHandler_Preview_MissingSKNo(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil, cfg: nil}
	body := strings.NewReader(`{"sk_no":"","tgl_berlaku":"2024-01-01","rows":[{"id_tt":"TT001"}]}`)
	c, w := createTestContext("POST", "/api/sk/preview", body)

	h.handlePostSKPreview(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview harus mengembalikan 400 jika rows kosong
func TestSKHandler_Preview_EmptyRows(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil, cfg: nil}
	body := strings.NewReader(`{"sk_no":"SK/001/2024","tgl_berlaku":"2024-01-01","rows":[]}`)
	c, w := createTestContext("POST", "/api/sk/preview", body)

	h.handlePostSKPreview(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// handlePostSKPreview — sukses mengembalikan preview data
func TestSKHandler_Preview_Success(t *testing.T) {
	t.Parallel()

	h := &SKHandler{repo: nil, cfg: nil}
	body := strings.NewReader(`{"sk_no":"SK/001/2024","tgl_berlaku":"2024-01-01","rows":[{"id_tt":"TT001","nama_tt":"VIP A"}]}`)
	c, w := createTestContext("POST", "/api/sk/preview", body)

	h.handlePostSKPreview(c)

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

	h := &SKHandler{repo: nil, cfg: nil}
	body := strings.NewReader("<<<bukan json>>>")
	c, w := createTestContext("POST", "/api/sk/import", body)

	h.handlePostSKImport(c)

	if w.Code != http.StatusBadRequest {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusBadRequest)
	}
}

// ─── Table-driven tests: writeJSON dengan berbagai status code ────────────────

func TestWriteJSON_TableDriven(t *testing.T) {
	t.Parallel()

	cases := []struct {
		name   string
		status int
		data   interface{}
	}{
		{"ok_with_map", http.StatusOK, map[string]string{"result": "ok"}},
		{"created", http.StatusCreated, map[string]int{"id": 42}},
		{"bad_request", http.StatusBadRequest, map[string]string{"error": "invalid input"}},
		{"internal_error", http.StatusInternalServerError, map[string]string{"error": "db error"}},
		{"conflict", http.StatusConflict, map[string]string{"status": "busy"}},
	}

	for _, tc := range cases {
		tc := tc
		t.Run(tc.name, func(t *testing.T) {
			t.Parallel()
			c, w := createTestContext("GET", "/test", nil)
			writeJSON(c, tc.status, tc.data)

			if w.Code != tc.status {
				t.Errorf("status: got %d, want %d", w.Code, tc.status)
			}
			assertContentTypeJSON(t, c)
		})
	}
}

// ─── APIHandler tests ──────────────────────────────────────────────────────────

// dispatcherInterface matches worker.Dispatcher for testability
type dispatcherInterface interface {
	TriggerManual() bool
	IsRunning() bool
	Stop()
}

// mockDispatcher implements dispatcherInterface for handler tests
type mockDispatcher struct {
	triggerManualFunc func() bool
	isRunningFunc     func() bool
}

func (m *mockDispatcher) TriggerManual() bool {
	if m.triggerManualFunc != nil {
		return m.triggerManualFunc()
	}
	return false
}

func (m *mockDispatcher) IsRunning() bool {
	if m.isRunningFunc != nil {
		return m.isRunningFunc()
	}
	return false
}

func (m *mockDispatcher) Stop() {}

// apiHandlerForTest wraps APIHandler with a mock dispatcher
type apiHandlerForTest struct {
	*APIHandler
	mockDisp *mockDispatcher
}

func newAPIHandlerForTest(mockDisp *mockDispatcher) *apiHandlerForTest {
	cfg := &config.Config{
		Security:    config.SecurityConfig{DashboardOrigin: "*"},
		Operational: config.OperationalConfig{LogFile: "test.log"},
	}
	h := &APIHandler{cfg: cfg}
	// We can't actually replace the dispatcher since it's a *worker.Dispatcher
	// but we'll use the mock for tests that don't need the real dispatcher
	return &apiHandlerForTest{APIHandler: h, mockDisp: mockDisp}
}

func TestHandleHealthz_Success(t *testing.T) {
	t.Parallel()

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}},
		dispatcher: nil, // not needed for healthz
	}
	c, w := createTestContext("GET", "/api/healthz", nil)

	h.handleHealthz(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertContentTypeJSON(t, c)

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["status"] != "ok" {
		t.Errorf("status: got %q, want %q", got["status"], "ok")
	}
}

func TestHandleSKActive_Success(t *testing.T) {
	t.Parallel()

	// Set global state
	worker.SetActiveSKNo("SK/TEST/2024")

	// Clean up after test
	defer worker.SetActiveSKNo("")

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}},
		dispatcher: nil, // not needed
	}
	c, w := createTestContext("GET", "/api/sk-active", nil)

	h.handleSKActive(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertContentTypeJSON(t, c)

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["sk_no"] != "SK/TEST/2024" {
		t.Errorf("sk_no: got %q, want %q", got["sk_no"], "SK/TEST/2024")
	}
}

func TestHandleSKActive_Empty(t *testing.T) {
	t.Parallel()

	// Set empty global state
	worker.SetActiveSKNo("")

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}},
		dispatcher: nil,
	}
	c, w := createTestContext("GET", "/api/sk-active", nil)

	h.handleSKActive(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["sk_no"] != "" {
		t.Errorf("sk_no: got %q, want empty string", got["sk_no"])
	}
}

func TestHandleGetBeds_Success(t *testing.T) {
	t.Parallel()

	// Set global beds state
	worker.SetBeds([]repository.BedSiranap{
		{IDTTSiranap: "TT-01", Siranap: "ICU", JmlRuang: 2, Jumlah: 10, Terisi: 5},
		{IDTTSiranap: "TT-02", Siranap: "OK", JmlRuang: 1, Jumlah: 5, Terisi: 2},
	})

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}},
		dispatcher: nil,
	}
	c, w := createTestContext("GET", "/api/beds", nil)

	h.handleGetBeds(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertContentTypeJSON(t, c)

	var got []repository.BedSiranap
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if len(got) != 2 {
		t.Errorf("beds count: got %d, want 2", len(got))
	}
}

func TestHandleGetLogs_Success(t *testing.T) {
	t.Parallel()

	// Create temp log file
	tmpDir := t.TempDir()
	logPath := tmpDir + "/test.log"

	// Write some log lines
	content := "Line 1\nLine 2\nLine 3\n"
	if err := os.WriteFile(logPath, []byte(content), 0644); err != nil {
		t.Fatalf("gagal membuat file test log: %v", err)
	}

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}, Operational: config.OperationalConfig{LogFile: logPath}},
		dispatcher: nil,
	}
	c, w := createTestContext("GET", "/api/logs", nil)

	h.handleGetLogs(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertContentTypeJSON(t, c)

	var got map[string]interface{}
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	lines, ok := got["lines"].([]interface{})
	if !ok {
		t.Fatalf("lines field missing or wrong type")
	}
	if len(lines) != 3 {
		t.Errorf("lines count: got %d, want 3", len(lines))
	}
}

func TestHandleGetLogs_FileNotExist(t *testing.T) {
	t.Parallel()

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}, Operational: config.OperationalConfig{LogFile: "/nonexistent/path/test.log"}},
		dispatcher: nil,
	}
	c, w := createTestContext("GET", "/api/logs", nil)

	h.handleGetLogs(c)

	// Should return 200 with empty lines, not 500
	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d ( ReadLast handles missing file gracefully)", w.Code, http.StatusOK)
	}
}

func TestHandleWorkerStatus_Idle(t *testing.T) {
	t.Parallel()

	// Ensure worker is not running
	worker.SetRunningFlag(false)

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}},
		dispatcher: nil, // not needed since we use global runningFlag
	}
	c, w := createTestContext("GET", "/api/worker/status", nil)

	h.handleWorkerStatus(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}
	assertContentTypeJSON(t, c)

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["status"] != "Idle" {
		t.Errorf("status: got %q, want %q", got["status"], "Idle")
	}
}

func TestHandleWorkerStatus_Running(t *testing.T) {
	t.Parallel()

	// Simulate worker running
	worker.SetRunningFlag(true)
	defer worker.SetRunningFlag(false)

	h := &APIHandler{
		cfg:        &config.Config{Security: config.SecurityConfig{DashboardOrigin: "*"}},
		dispatcher: nil,
	}
	c, w := createTestContext("GET", "/api/worker/status", nil)

	h.handleWorkerStatus(c)

	if w.Code != http.StatusOK {
		t.Errorf("status: got %d, want %d", w.Code, http.StatusOK)
	}

	var got map[string]string
	if err := json.NewDecoder(w.Body).Decode(&got); err != nil {
		t.Fatalf("gagal decode body JSON: %v", err)
	}
	if got["status"] != "Running" {
		t.Errorf("status: got %q, want %q", got["status"], "Running")
	}
}
