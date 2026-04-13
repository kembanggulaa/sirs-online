package worker

import (
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"sirs-online/config"
	"sirs-online/internal/logger"
	"sirs-online/internal/repository"
)

func init() {
	// Initialize dummy logger to prevent panics during test
	_ = logger.Init("./test.log")
}

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
