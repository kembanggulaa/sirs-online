package handler

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"sirs-online/internal/repository"
)

func TestBedsHandler_GetRooms(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mockFunc   func(ctx context.Context) ([]string, error)
		wantStatus int
	}{
		{
			name: "Success 200",
			mockFunc: func(ctx context.Context) ([]string, error) {
				return []string{"VIP", "Reguler"}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Internal Server Error 500",
			mockFunc: func(ctx context.Context) ([]string, error) {
				return nil, errors.New("db disconnect")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockRepo := &MockBedsRepository{GetDistinctClassRoomsFunc: tt.mockFunc}
			h := NewBedsHandler(mockRepo, nil)

			req := httptest.NewRequest("GET", "/api/beds/rooms", nil)
			w := httptest.NewRecorder()

			h.handleGetRooms(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status got %d; want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestBedsHandler_UpsertBeds_Integration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		mockFunc   func(ctx context.Context, req repository.BedsUpsertRequest) (repository.UpsertResult, error)
		wantStatus int
	}{
		{
			name: "Success 200",
			body: `{"class_room_id":"VIP","rows":[{"bed_id":1,"nm_kelas":"VIP","id_kelas":"1"}]}`,
			mockFunc: func(ctx context.Context, req repository.BedsUpsertRequest) (repository.UpsertResult, error) {
				return repository.UpsertResult{Saved: 1, Inserted: 1}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Internal Server Error 500",
			body: `{"class_room_id":"VIP","rows":[{"bed_id":1,"nm_kelas":"VIP","id_kelas":"1"}]}`,
			mockFunc: func(ctx context.Context, req repository.BedsUpsertRequest) (repository.UpsertResult, error) {
				return repository.UpsertResult{}, errors.New("conflict")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockRepo := &MockBedsRepository{UpsertBedsFunc: tt.mockFunc}
			h := NewBedsHandler(mockRepo, nil)

			req := httptest.NewRequest("POST", "/api/beds/upsert", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.handlePostUpsertBeds(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status got %d; want %d", w.Code, tt.wantStatus)
			}

			// jika success pastikan format hasil sesuai dengan respons JSON
			if tt.wantStatus == http.StatusOK {
				var res repository.UpsertResult
				err := json.NewDecoder(w.Body).Decode(&res)
				if err != nil {
					t.Fatalf("could not decode json: %v", err)
				}
				if res.Saved != 1 {
					t.Errorf("expected saved=1, got %d", res.Saved)
				}
			}
		})
	}
}
