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

func TestSKHandler_GetSKList(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		mockFunc   func(ctx context.Context) ([]string, error)
		wantStatus int
	}{
		{
			name: "Success 200",
			mockFunc: func(ctx context.Context) ([]string, error) {
				return []string{"SK001", "SK002"}, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Internal Server Error 500",
			mockFunc: func(ctx context.Context) ([]string, error) {
				return nil, errors.New("timeout")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockRepo := &MockSKRepository{GetSKListFunc: tt.mockFunc}
			h := NewSKHandler(mockRepo, nil)

			req := httptest.NewRequest("GET", "/api/sk/list", nil)
			w := httptest.NewRecorder()

			h.handleGetSKList(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status got %d; want %d", w.Code, tt.wantStatus)
			}
		})
	}
}

func TestSKHandler_Import_Integration(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name       string
		body       string
		mockFunc   func(ctx context.Context, req repository.SKImportRequest) (int, error)
		wantStatus int
	}{
		{
			name: "Success 200",
			body: `{"sk_no":"SK001","tgl_berlaku":"2024-01-01","rows":[{"id_tt_siranap":"TT1"}]}`,
			mockFunc: func(ctx context.Context, req repository.SKImportRequest) (int, error) {
				return 1, nil
			},
			wantStatus: http.StatusOK,
		},
		{
			name: "Internal Server Error 500",
			body: `{"sk_no":"SK001","tgl_berlaku":"2024-01-01","rows":[{"id_tt_siranap":"TT1"}]}`,
			mockFunc: func(ctx context.Context, req repository.SKImportRequest) (int, error) {
				return 0, errors.New("deadlock")
			},
			wantStatus: http.StatusInternalServerError,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			mockRepo := &MockSKRepository{BulkInsertSKBedFunc: tt.mockFunc}
			h := NewSKHandler(mockRepo, nil)

			req := httptest.NewRequest("POST", "/api/sk/import", strings.NewReader(tt.body))
			w := httptest.NewRecorder()

			h.handlePostSKImport(w, req)

			if w.Code != tt.wantStatus {
				t.Errorf("status got %d; want %d", w.Code, tt.wantStatus)
			}

			// Validate success payload mapped correctly
			if tt.wantStatus == http.StatusOK {
				var res map[string]interface{}
				_ = json.NewDecoder(w.Body).Decode(&res)
				if int(res["inserted"].(float64)) != 1 {
					t.Errorf("expected inserted=1, got %v", res["inserted"])
				}
			}
		})
	}
}
