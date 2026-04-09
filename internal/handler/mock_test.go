package handler

import (
	"context"

	"sirs-online/internal/repository"
)

// ─── Mocking untuk Beds ───────────────────────────────────────────────────────

type MockBedsRepository struct {
	GetDistinctClassRoomsFunc func(ctx context.Context) ([]string, error)
	GetKamarByClassRoomFunc   func(ctx context.Context, classRoomID string) ([]string, error)
	GetBedsByRoomFunc         func(ctx context.Context, classRoomID string) (repository.BedsRoomResult, error)
	UpsertBedsFunc            func(ctx context.Context, req repository.BedsUpsertRequest) (repository.UpsertResult, error)
}

func (m *MockBedsRepository) GetDistinctClassRooms(ctx context.Context) ([]string, error) {
	return m.GetDistinctClassRoomsFunc(ctx)
}

func (m *MockBedsRepository) GetKamarByClassRoom(ctx context.Context, classRoomID string) ([]string, error) {
	return m.GetKamarByClassRoomFunc(ctx, classRoomID)
}

func (m *MockBedsRepository) GetBedsByRoom(ctx context.Context, classRoomID string) (repository.BedsRoomResult, error) {
	return m.GetBedsByRoomFunc(ctx, classRoomID)
}

func (m *MockBedsRepository) UpsertBeds(ctx context.Context, req repository.BedsUpsertRequest) (repository.UpsertResult, error) {
	return m.UpsertBedsFunc(ctx, req)
}

// ─── Mocking untuk SK ─────────────────────────────────────────────────────────

type MockSKRepository struct {
	BulkInsertSKBedFunc func(ctx context.Context, req repository.SKImportRequest) (int, error)
	GetSKListFunc       func(ctx context.Context) ([]string, error)
	GetSKDetailFunc     func(ctx context.Context, skNo string) ([]repository.SKBedRow, error)
}

func (m *MockSKRepository) BulkInsertSKBed(ctx context.Context, req repository.SKImportRequest) (int, error) {
	return m.BulkInsertSKBedFunc(ctx, req)
}

func (m *MockSKRepository) GetSKList(ctx context.Context) ([]string, error) {
	return m.GetSKListFunc(ctx)
}

func (m *MockSKRepository) GetSKDetail(ctx context.Context, skNo string) ([]repository.SKBedRow, error) {
	return m.GetSKDetailFunc(ctx, skNo)
}
