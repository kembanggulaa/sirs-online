package repository

import (
	"context"
	"database/sql"
	"fmt"
	"os"
	"testing"

	_ "github.com/microsoft/go-mssqldb"
)

// TestGetBedsByRoom_Integration tests the core functionality of Tab 6 Beds Management
func TestGetBedsByRoom_Integration(t *testing.T) {
	// Skip if no test database configured
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping integration test")
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		t.Fatalf("Failed to ping database: %v", err)
	}

	ctx := context.Background()
	repo := NewBedsRepository(db, "TEST_ORG")

	t.Run("GetDistinctClassRooms should return active class rooms", func(t *testing.T) {
		rooms, err := repo.GetDistinctClassRooms(ctx)
		if err != nil {
			t.Fatalf("GetDistinctClassRooms() error = %v", err)
		}

		t.Logf("Found %d distinct class rooms", len(rooms))
		for i, room := range rooms {
			t.Logf("  [%d] class_room_id = %q", i, room)
		}

		if len(rooms) == 0 {
			t.Log("WARNING: No active class rooms found in sk_bed table")
		}
	})

	// Test each class room individually
	t.Run("GetBedsByRoom for each class room", func(t *testing.T) {
		rooms, err := repo.GetDistinctClassRooms(ctx)
		if err != nil {
			t.Fatalf("Failed to get class rooms: %v", err)
		}

		for _, roomID := range rooms {
			t.Run(fmt.Sprintf("class_room_id=%s", roomID), func(t *testing.T) {
				result, err := repo.GetBedsByRoom(ctx, roomID)
				if err != nil {
					t.Fatalf("GetBedsByRoom(%q) error = %v", roomID, err)
				}

				t.Logf("Mode: %s", result.Mode)
				t.Logf("Number of kamar groups: %d", len(result.Kamars))

				for i, kamar := range result.Kamars {
					t.Logf("  [%d] Kamar = %q", i, kamar.Kamar)
					t.Logf("      Defaults: id_tt_siranap=%q, covid=%q, id_kelas=%q, nm_kelas=%q",
						kamar.Defaults["id_tt_siranap"],
						kamar.Defaults["covid"],
						kamar.Defaults["id_kelas"],
						kamar.Defaults["nm_kelas"])
					t.Logf("      Rows count: %d", len(kamar.Rows))

					for j, row := range kamar.Rows {
						t.Logf("        [%d] bed_id=%d, room_id=%q, id_kelas=%q, nm_kelas=%q",
							j, row.BedID, row.RoomID, row.IDKelas, row.NmKelas)
					}
				}

				// Validate that at least one kamar group exists
				if len(result.Kamars) == 0 {
					t.Logf("WARNING: No kamar groups returned for class_room_id=%q", roomID)
					t.Log("This could indicate:")
					t.Log("  1. No records in sk_bed with tgl_berakhir IS NULL")
					t.Log("  2. Mismatch between sk_bed.kamar and beds.kamar values")
					t.Log("  3. Empty/NULL kamar values causing grouping issues")
				}
			})
		}
	})
}

// TestGetBedsByRoom_EmptyClassRoom tests behavior with non-existent class room
func TestGetBedsByRoom_EmptyClassRoom(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping integration test")
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := NewBedsRepository(db, "TEST_ORG")

	t.Run("GetBedsByRoom with non-existent class_room_id", func(t *testing.T) {
		result, err := repo.GetBedsByRoom(ctx, "NON_EXISTENT_ROOM_99999")
		if err != nil {
			t.Fatalf("GetBedsByRoom() unexpected error = %v", err)
		}

		if result.Mode != "new" {
			t.Errorf("Expected mode='new' for empty room, got %q", result.Mode)
		}

		if len(result.Kamars) != 0 {
			t.Errorf("Expected 0 kamars for non-existent room, got %d", len(result.Kamars))
		}

		t.Log("PASS: Empty room returns correct default structure")
	})
}

// TestGetBedsByRoom_DataIntegrity checks for common data integrity issues
func TestGetBedsByRoom_DataIntegrity(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping integration test")
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()

	// Query to check for potential data issues
	t.Run("Check for sk_bed records with empty/NULL kamar", func(t *testing.T) {
		query := `
			SELECT class_room_id, kamar, namaruang, COUNT(*) as cnt
			FROM sk_bed WITH (NOLOCK)
			WHERE tgl_berakhir IS NULL
			GROUP BY class_room_id, kamar, namaruang
			ORDER BY class_room_id, kamar
		`
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		defer rows.Close()

		emptyKamarCount := 0
		for rows.Next() {
			var classRoomID, kamar, namaruang sql.NullString
			var cnt int
			if err := rows.Scan(&classRoomID, &kamar, &namaruang, &cnt); err != nil {
				t.Fatalf("Scan error: %v", err)
			}

			kamarVal := ""
			if kamar.Valid && kamar.String != "" {
				kamarVal = kamar.String
			}

			if kamarVal == "" {
				emptyKamarCount++
				namaruangVal := ""
				if namaruang.Valid {
					namaruangVal = namaruang.String
				}
				t.Logf("WARNING: class_room_id=%q has empty kamar, namaruang=%q (count=%d)",
					classRoomID.String, namaruangVal, cnt)
			}
		}

		if emptyKamarCount > 0 {
			t.Logf("Found %d sk_bed groups with empty kamar values", emptyKamarCount)
			t.Log("This may cause accordion grouping issues if namaruang is also empty")
		} else {
			t.Log("PASS: All sk_bed records have non-empty kamar values")
		}
	})

	t.Run("Check for beds records with empty/NULL kamar", func(t *testing.T) {
		query := `
			SELECT class_room_id, kamar, COUNT(*) as cnt
			FROM beds WITH (NOLOCK)
			GROUP BY class_room_id, kamar
			ORDER BY class_room_id, kamar
		`
		rows, err := db.QueryContext(ctx, query)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		defer rows.Close()

		emptyKamarCount := 0
		for rows.Next() {
			var classRoomID, kamar sql.NullString
			var cnt int
			if err := rows.Scan(&classRoomID, &kamar, &cnt); err != nil {
				t.Fatalf("Scan error: %v", err)
			}

			kamarVal := ""
			if kamar.Valid && kamar.String != "" {
				kamarVal = kamar.String
			}

			if kamarVal == "" {
				emptyKamarCount += cnt
				t.Logf("WARNING: class_room_id=%q has %d beds with empty kamar",
					classRoomID.String, cnt)
			}
		}

		if emptyKamarCount > 0 {
			t.Logf("Found %d beds records with empty kamar values", emptyKamarCount)
			t.Log("These beds will be assigned to first available sk_bed group")
		} else {
			t.Log("PASS: All beds records have non-empty kamar values")
		}
	})

	t.Run("Check for mismatched kamar values between sk_bed and beds", func(t *testing.T) {
		// Get all active class rooms
		roomsQuery := `SELECT DISTINCT class_room_id FROM sk_bed WITH (NOLOCK) WHERE tgl_berakhir IS NULL`
		roomsRows, err := db.QueryContext(ctx, roomsQuery)
		if err != nil {
			t.Fatalf("Query error: %v", err)
		}
		defer roomsRows.Close()

		var classRooms []string
		for roomsRows.Next() {
			var cr sql.NullString
			if err := roomsRows.Scan(&cr); err != nil {
				t.Fatalf("Scan error: %v", err)
			}
			if cr.Valid && cr.String != "" {
				classRooms = append(classRooms, cr.String)
			}
		}

		mismatchCount := 0
		for _, roomID := range classRooms {
			// Get kamar values from sk_bed
			skQuery := `
				SELECT DISTINCT ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key
				FROM sk_bed WITH (NOLOCK)
				WHERE class_room_id = @p1 AND tgl_berakhir IS NULL
			`
			skRows, err := db.QueryContext(ctx, skQuery, roomID)
			if err != nil {
				t.Logf("Failed to query sk_bed for %s: %v", roomID, err)
				continue
			}

			skKamars := make(map[string]bool)
			for skRows.Next() {
				var k sql.NullString
				if err := skRows.Scan(&k); err != nil {
					t.Logf("Scan error: %v", err)
					continue
				}
				if k.Valid {
					skKamars[k.String] = true
				}
			}
			skRows.Close()

			// Get kamar values from beds
			bedsQuery := `
				SELECT DISTINCT ISNULL(kamar, '') as kamar_val
				FROM beds WITH (NOLOCK)
				WHERE class_room_id = @p1
			`
			bedsRows, err := db.QueryContext(ctx, bedsQuery, roomID)
			if err != nil {
				t.Logf("Failed to query beds for %s: %v", roomID, err)
				continue
			}

			bedsKamars := make(map[string]bool)
			for bedsRows.Next() {
				var k sql.NullString
				if err := bedsRows.Scan(&k); err != nil {
					t.Logf("Scan error: %v", err)
					continue
				}
				bedsKamars[k.String] = true
			}
			bedsRows.Close()

			// Check for mismatches
			for bedsKamar := range bedsKamars {
				if !skKamars[bedsKamar] {
					mismatchCount++
					if mismatchCount <= 10 { // Limit output
						t.Logf("MISMATCH: class_room_id=%q, beds.kamar=%q not found in sk_bed",
							roomID, bedsKamar)
					}
				}
			}
		}

		if mismatchCount > 0 {
			t.Logf("Found %d mismatched kamar values between beds and sk_bed", mismatchCount)
			t.Log("These mismatches cause new accordion groups to be created")
		} else {
			t.Log("PASS: All kamar values match between sk_bed and beds")
		}
	})
}

// TestGetKamarByClassRoom tests the kamar filtering functionality
func TestGetKamarByClassRoom_Integration(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping integration test")
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := NewBedsRepository(db, "TEST_ORG")

	t.Run("GetKamarByClassRoom for each active room", func(t *testing.T) {
		rooms, err := repo.GetDistinctClassRooms(ctx)
		if err != nil {
			t.Fatalf("Failed to get class rooms: %v", err)
		}

		for _, roomID := range rooms {
			t.Run(fmt.Sprintf("class_room_id=%s", roomID), func(t *testing.T) {
				kamars, err := repo.GetKamarByClassRoom(ctx, roomID)
				if err != nil {
					t.Fatalf("GetKamarByClassRoom(%q) error = %v", roomID, err)
				}

				t.Logf("Found %d kamar(s) for class_room_id=%q", len(kamars), roomID)
				for i, kamar := range kamars {
					t.Logf("  [%d] %q", i, kamar)
				}
			})
		}
	})
}

// TestUpsertBeds_BasicOperation tests basic upsert functionality
func TestUpsertBeds_BasicOperation(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_DSN")
	if dsn == "" {
		t.Skip("TEST_DATABASE_DSN not set, skipping integration test")
	}

	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		t.Fatalf("Failed to connect to database: %v", err)
	}
	defer db.Close()

	ctx := context.Background()
	repo := NewBedsRepository(db, "TEST_ORG")

	t.Run("UpsertBeds with minimal valid data", func(t *testing.T) {
		// First, get an existing class room
		rooms, err := repo.GetDistinctClassRooms(ctx)
		if err != nil || len(rooms) == 0 {
			t.Skip("No class rooms available for testing")
		}

		classRoomID := rooms[0]

		// Get existing beds to use a valid bed_id
		result, err := repo.GetBedsByRoom(ctx, classRoomID)
		if err != nil {
			t.Fatalf("Failed to get beds: %v", err)
		}

		// Create upsert request
		req := BedsUpsertRequest{
			ClassRoomID: classRoomID,
			Rows:        []BedRow{},
		}

		// If there are existing beds, use one; otherwise create a new one
		if len(result.Kamars) > 0 && len(result.Kamars[0].Rows) > 0 {
			existingBed := result.Kamars[0].Rows[0]
			req.Rows = append(req.Rows, BedRow{
				BedID:            existingBed.BedID,
				Kamar:            existingBed.Kamar,
				RoomID:           existingBed.RoomID,
				IDKelas:          result.Kamars[0].Defaults["id_kelas"],
				NmKelas:          result.Kamars[0].Defaults["nm_kelas"],
				IDPerawatan:      existingBed.IDPerawatan,
				NmPerawatan:      existingBed.NmPerawatan,
				IDTTSiranap:      result.Kamars[0].Defaults["id_tt_siranap"],
				IDSiranap:        existingBed.IDSiranap,
				DeskripsiSiranap: existingBed.DeskripsiSiranap,
				Covid:            result.Kamars[0].Defaults["covid"],
			})
		} else {
			// Create a new bed with bed_id=999999 (unlikely to exist)
			req.Rows = append(req.Rows, BedRow{
				BedID:       999999,
				Kamar:       "Test Kamar",
				RoomID:      "ROOM001",
				IDKelas:     "1",
				NmKelas:     "Test Kelas",
				IDPerawatan: "PER001",
				NmPerawatan: "Test Perawatan",
				IDTTSiranap: "TT001",
				IDSiranap:   "SIR001",
				Covid:       "0",
			})
		}

		upsertResult, err := repo.UpsertBeds(ctx, req)
		if err != nil {
			t.Fatalf("UpsertBeds() error = %v", err)
		}

		t.Logf("Upsert result: saved=%d, inserted=%d, updated=%d, deleted=%d",
			upsertResult.Saved, upsertResult.Inserted, upsertResult.Updated, upsertResult.Deleted)

		if upsertResult.Saved == 0 && upsertResult.Deleted == 0 {
			t.Error("Expected at least one row to be saved or deleted")
		}
	})
}
