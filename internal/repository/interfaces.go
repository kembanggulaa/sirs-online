package repository

import (
	"context"
)

// BedsRepositoryInterface mendefinisikan kontrak operasi pada fitur Manajemen Beds
type BedsRepositoryInterface interface {
	GetDistinctClassRooms(ctx context.Context) ([]string, error)
	GetKamarByClassRoom(ctx context.Context, classRoomID string) ([]string, error)
	GetBedsByRoom(ctx context.Context, classRoomID string) (BedsRoomResult, error)
	UpsertBeds(ctx context.Context, req BedsUpsertRequest) (UpsertResult, error)
}

// SKRepositoryInterface mendefinisikan kontrak operasi pada fitur Manajemen SK
type SKRepositoryInterface interface {
	BulkInsertSKBed(ctx context.Context, req SKImportRequest) (int, error)
	GetSKList(ctx context.Context) ([]string, error)
	GetSKDetail(ctx context.Context, skNo string) ([]SKBedRow, error)
}

// BedRepositoryInterface mendefinisikan kontrak operasi untuk Worker Sync Kemenkes
type BedRepositoryInterface interface {
	GetActiveSKNo() (string, error)
	GetBedAvailability(skNo string) ([]BedSiranap, error)
}
