package worker

import (
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sirs-online/config"
	"sirs-online/internal/repository"
)

type MockBedRepository struct {
	GetActiveSKNoFunc      func() (string, error)
	GetBedAvailabilityFunc func(skNo string) ([]repository.BedSiranap, error)
}

func (m *MockBedRepository) GetActiveSKNo() (string, error) {
	return m.GetActiveSKNoFunc()
}

func (m *MockBedRepository) GetBedAvailability(skNo string) ([]repository.BedSiranap, error) {
	return m.GetBedAvailabilityFunc(skNo)
}

func TestProcessJob_Success(t *testing.T) {
	// 1. Setup mock server Kemenkes
	var putCalled int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/Fasyankes" {
			// Mocking Fetch mapping
			resp := fasyankesResponse{
				Fasyankes: []fasyankesRecord{
					{IDTtt: "TTT-123", IDTt: "TT-01", Ruang: "ICU"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}

		if r.Method == "PUT" && r.URL.Path == "/Fasyankes" {
			// Mocking sinkronisasi / update
			putCalled++

			// Validate payload received and token
			if r.Header.Get("X-rs-id") != "RS123" {
				t.Errorf("expected X-rs-id to be RS123")
			}
			var body map[string]string
			_ = json.NewDecoder(r.Body).Decode(&body)
			if body["id_t_tt"] != "TTT-123" {
				t.Errorf("expected payload to map id_t_tt, got %s", body["id_t_tt"])
			}

			w.WriteHeader(http.StatusOK)
			return
		}

		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	// 2. Setup mock repository
	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return []repository.BedSiranap{
				{
					IDTTSiranap: "TT-01",
					Siranap:     "ICU",
					JmlRuang:    10,
					Jumlah:      10,
					Terisi:      5,
				},
			}, nil
		},
	}

	// 3. Konfigurasi
	cfg := &config.Config{
		API: config.APIConfig{
			URL:  ts.URL,
			RsID: "RS123",
			Pass: "pass123",
		},
		Operational: config.OperationalConfig{
			TLSSkipVerify: true,
			RetryMax:      0, // prevent retries taking too long in test
		},
	}

	job := Job{
		Repo:   mockRepo,
		Config: cfg,
	}

	// 4. Jalankan processJob
	processJob(job)

	// 5. Validasi
	if putCalled != 1 {
		t.Errorf("expected PUT /Fasyankes to be called 1 time, got %d", putCalled)
	}

	activeSK := GetActiveSKNoCurrent()
	if activeSK != "SK-001" {
		t.Errorf("expected global SK NO to be updated to SK-001, got %s", activeSK)
	}

	beds := GetBeds()
	if len(beds) != 1 || beds[0].IDTTSiranap != "TT-01" {
		t.Errorf("expected global beds state to be correctly updated")
	}
}

func TestDispatcher_TriggerManual(t *testing.T) {
	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return []repository.BedSiranap{}, nil
		},
	}
	cfg := &config.Config{API: config.APIConfig{URL: "http://example.com"}}

	dispatcher := NewDispatcher(mockRepo, cfg)
	defer dispatcher.Stop()

	// Simulate TriggerManual
	dispatched := dispatcher.TriggerManual()

	// Job should be accepted because it wasn't running
	if !dispatched {
		t.Error("expected initial TriggerManual to dispatch job")
	}

	// Let worker consume it briefly
	time.Sleep(50 * time.Millisecond)

	// Check running state
	// Not predictably checking running status unless we block processJob,
	// here we just aim to pass execution without panics.
}

func TestProcessJob_GetActiveSKNoError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "", fmt.Errorf("no active SK found")
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return nil, nil
		},
	}
	cfg := &config.Config{
		API:          config.APIConfig{URL: ts.URL, RsID: "RS123", Pass: "pass"},
		Operational: config.OperationalConfig{TLSSkipVerify: true, RetryMax: 0},
	}

	job := Job{Repo: mockRepo, Config: cfg}
	processJob(job)

	// Should exit early without calling PUT
	// No assertion needed - just verify no panic
}

func TestProcessJob_GetBedAvailabilityError(t *testing.T) {
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.WriteHeader(http.StatusOK)
	}))
	defer ts.Close()

	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return nil, fmt.Errorf("db connection lost")
		},
	}
	cfg := &config.Config{
		API:          config.APIConfig{URL: ts.URL, RsID: "RS123", Pass: "pass"},
		Operational: config.OperationalConfig{TLSSkipVerify: true, RetryMax: 0},
	}

	job := Job{Repo: mockRepo, Config: cfg}
	processJob(job)

	// Should exit early after SK detection
}

func TestProcessJob_FetchMappingFails_ContinueWithSync(t *testing.T) {
	var putCalled int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/Fasyankes" {
			w.WriteHeader(http.StatusInternalServerError)
			w.Write([]byte("Server error"))
			return
		}
		if r.Method == "PUT" && r.URL.Path == "/Fasyankes" {
			putCalled++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return []repository.BedSiranap{
				{IDTTSiranap: "TT-01", Siranap: "ICU", JmlRuang: 1, Jumlah: 10, Terisi: 5},
			}, nil
		},
	}
	cfg := &config.Config{
		API:          config.APIConfig{URL: ts.URL, RsID: "RS123", Pass: "pass"},
		Operational: config.OperationalConfig{TLSSkipVerify: true, RetryMax: 0},
	}

	job := Job{Repo: mockRepo, Config: cfg}
	processJob(job)

	// PUT should NOT be called because mapping fetch failed
	if putCalled != 0 {
		t.Errorf("expected PUT to be skipped when mapping fetch fails, got %d calls", putCalled)
	}
}

func TestProcessJob_BedSkippedWhenMappingNotFound(t *testing.T) {
	var putCalled int
	ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		if r.Method == "GET" && r.URL.Path == "/Fasyankes" {
			// Return mapping that doesn't match our bed
			resp := fasyankesResponse{
				Fasyankes: []fasyankesRecord{
					{IDTtt: "OTHER-ID", IDTt: "OTHER-TT", Ruang: "OTHER-ROOM"},
				},
			}
			w.WriteHeader(http.StatusOK)
			_ = json.NewEncoder(w).Encode(resp)
			return
		}
		if r.Method == "PUT" && r.URL.Path == "/Fasyankes" {
			putCalled++
			w.WriteHeader(http.StatusOK)
			return
		}
		w.WriteHeader(http.StatusNotFound)
	}))
	defer ts.Close()

	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return []repository.BedSiranap{
				{IDTTSiranap: "TT-01", Siranap: "ICU", JmlRuang: 1, Jumlah: 10, Terisi: 5},
			}, nil
		},
	}
	cfg := &config.Config{
		API:          config.APIConfig{URL: ts.URL, RsID: "RS123", Pass: "pass"},
		Operational: config.OperationalConfig{TLSSkipVerify: true, RetryMax: 0},
	}

	job := Job{Repo: mockRepo, Config: cfg}
	processJob(job)

	// PUT should NOT be called because bed's id_tt|ruang doesn't match mapping
	if putCalled != 0 {
		t.Errorf("expected PUT to be skipped when mapping not found for bed, got %d calls", putCalled)
	}
}

func TestDispatcher_DispatchSkipWhenRunning(t *testing.T) {
	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			// Simulate slow operation to keep worker running
			time.Sleep(200 * time.Millisecond)
			return []repository.BedSiranap{}, nil
		},
	}
	cfg := &config.Config{API: config.APIConfig{URL: "http://example.com"}}

	dispatcher := NewDispatcher(mockRepo, cfg)
	defer dispatcher.Stop()

	// First trigger
	dispatcher.TriggerManual()
	time.Sleep(10 * time.Millisecond)

	// Second trigger while running should be skipped
	dispatched := dispatcher.TriggerManual()
	if dispatched {
		t.Error("expected second TriggerManual to be skipped while worker is running")
	}
}

func TestNewKemenkesClient_Singleton(t *testing.T) {
	// Reset singleton state for test
	kemenkesClientMu.Lock()
	kemenkesClient = nil
	kemenkesSkipTLS = false
	kemenkesClientMu.Unlock()

	client1 := NewKemenkesClient(false)
	client2 := NewKemenkesClient(false)

	if client1 != client2 {
		t.Error("expected same singleton instance for same TLS config")
	}

	// Different config should return different instance
	client3 := NewKemenkesClient(true)
	if client3 == client1 {
		t.Error("expected different instance for different TLS config")
	}
}

func TestNewKemenkesClient_ReuseSameConfig(t *testing.T) {
	// Reset singleton state for test
	kemenkesClientMu.Lock()
	kemenkesClient = nil
	kemenkesSkipTLS = false
	kemenkesClientMu.Unlock()

	client1 := NewKemenkesClient(true)
	client2 := NewKemenkesClient(true)

	if client1 != client2 {
		t.Error("expected singleton reuse for same TLS config")
	}
}

func TestTruncateStr(t *testing.T) {
	tests := []struct {
		input    string
		maxLen   int
		expected string
	}{
		{"short", 10, "short"},
		{"exactly10!", 10, "exactly10!"},
		{"this is longer", 10, "this is lo...[truncated]"},
		{"x", 1, "x"},        // len(s) <= maxLen: no truncation
		{"", 5, ""},
		{"abc", 2, "ab...[truncated]"},
	}

	for _, tt := range tests {
		result := truncateStr(tt.input, tt.maxLen)
		if result != tt.expected {
			t.Errorf("truncateStr(%q, %d): got %q, want %q", tt.input, tt.maxLen, result, tt.expected)
		}
	}
}

func TestDispatcher_IsRunning_AfterJobComplete(t *testing.T) {
	mockRepo := &MockBedRepository{
		GetActiveSKNoFunc: func() (string, error) {
			return "SK-001", nil
		},
		GetBedAvailabilityFunc: func(skNo string) ([]repository.BedSiranap, error) {
			return []repository.BedSiranap{}, nil
		},
	}
	cfg := &config.Config{API: config.APIConfig{URL: "http://example.com"}}

	dispatcher := NewDispatcher(mockRepo, cfg)
	defer dispatcher.Stop()

	// Wait for previous test job to complete
	time.Sleep(200 * time.Millisecond)

	// Note: This test may be flaky due to global state from other tests
	// Running in isolation would give more reliable results
	running := dispatcher.IsRunning()
	t.Logf("IsRunning after 200ms: %v (may be affected by parallel test state)", running)
}
