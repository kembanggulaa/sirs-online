package main

import (
	"database/sql"
	"fmt"
	"log"
	"os"

	_ "github.com/microsoft/go-mssqldb"
)

// Simple diagnostic tool to check Tab 6 Beds Management data
// Usage: go run simple_check.go
// Or set environment variables:
//   set DB_HOST=localhost
//   set DB_PORT=1433
//   set DB_USER=your_user
//   set DB_PASS=your_password
//   set DB_NAME=your_database
//   go run simple_check.go

func main() {
	fmt.Println("========================================")
	fmt.Println("Tab 6 Beds Management - Simple Data Check")
	fmt.Println("========================================")
	fmt.Println()

	// Get connection parameters from environment
	host := getEnv("DB_HOST", "localhost")
	port := getEnv("DB_PORT", "1433")
	user := getEnv("DB_USER", "")
	pass := getEnv("DB_PASS", "")
	dbName := getEnv("DB_NAME", "")

	if user == "" || pass == "" || dbName == "" {
		fmt.Println("ERROR: Database credentials not set!")
		fmt.Println("Please set these environment variables:")
		fmt.Println("  DB_HOST, DB_PORT, DB_USER, DB_PASS, DB_NAME")
		fmt.Println()
		fmt.Println("Or use the DSN format:")
		fmt.Println("  go run simple_check.go with TEST_DATABASE_DSN set")
		os.Exit(1)
	}

	// Build DSN
	dsn := fmt.Sprintf("server=%s;port=%s;user id=%s;password=%s;database=%s",
		host, port, user, pass, dbName)

	// Connect to database
	db, err := sql.Open("sqlserver", dsn)
	if err != nil {
		log.Fatalf("Failed to open database connection: %v", err)
	}
	defer db.Close()

	// Test connection
	if err := db.Ping(); err != nil {
		log.Fatalf("Failed to ping database: %v", err)
	}

	fmt.Println("✓ Database connection successful")
	fmt.Println()

	// 1. Check active class rooms
	fmt.Println("=== 1. Active Class Rooms (sk_bed) ===")
	rows, err := db.Query(`
		SELECT class_room_id, COUNT(*) as cnt
		FROM sk_bed WITH (NOLOCK)
		WHERE tgl_berakhir IS NULL
		GROUP BY class_room_id
		ORDER BY class_room_id
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		defer rows.Close()
		
		count := 0
		for rows.Next() {
			var roomID string
			var cnt int
			if err := rows.Scan(&roomID, &cnt); err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}
			fmt.Printf("  %-30s %d records\n", roomID, cnt)
			count++
		}
		
		if count == 0 {
			fmt.Println("  ⚠ WARNING: No active class rooms found!")
			fmt.Println("  This means all sk_bed records have tgl_berakhir set.")
		} else {
			fmt.Printf("  ✓ Found %d active class rooms\n", count)
		}
	}
	fmt.Println()

	// 2. Check for empty kamar issues
	fmt.Println("=== 2. Empty/NULL Kamar Check ===")
	rows2, err := db.Query(`
		SELECT 
			class_room_id,
			COUNT(*) as total,
			SUM(CASE WHEN ISNULL(kamar, '') = '' THEN 1 ELSE 0 END) as empty_kamar
		FROM sk_bed WITH (NOLOCK)
		WHERE tgl_berakhir IS NULL
		GROUP BY class_room_id
		HAVING SUM(CASE WHEN ISNULL(kamar, '') = '' THEN 1 ELSE 0 END) > 0
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		defer rows2.Close()
		
		hasIssue := false
		for rows2.Next() {
			var roomID string
			var total, emptyCnt int
			if err := rows2.Scan(&roomID, &total, &emptyCnt); err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}
			fmt.Printf("  ⚠ %-30s %d/%d records with empty kamar\n", roomID, emptyCnt, total)
			hasIssue = true
		}
		
		if !hasIssue {
			fmt.Println("  ✓ No empty kamar issues found")
		}
	}
	fmt.Println()

	// 3. Check beds table
	fmt.Println("=== 3. Beds Table Summary ===")
	rows3, err := db.Query(`
		SELECT TOP 10
			class_room_id,
			COUNT(*) as total_beds,
			COUNT(DISTINCT kamar) as distinct_kamar,
			SUM(CASE WHEN ISNULL(kamar, '') = '' THEN 1 ELSE 0 END) as empty_kamar
		FROM beds WITH (NOLOCK)
		GROUP BY class_room_id
		ORDER BY class_room_id
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		defer rows3.Close()
		
		count := 0
		for rows3.Next() {
			var roomID string
			var totalBeds, distinctKamar, emptyKamar int
			if err := rows3.Scan(&roomID, &totalBeds, &distinctKamar, &emptyKamar); err != nil {
				log.Printf("Scan error: %v", err)
				continue
			}
			fmt.Printf("  %-30s %d beds, %d kamar groups", roomID, totalBeds, distinctKamar)
			if emptyKamar > 0 {
				fmt.Printf(" (%d with empty kamar)", emptyKamar)
			}
			fmt.Println()
			count++
		}
		
		if count == 0 {
			fmt.Println("  No beds records found")
		}
	}
	fmt.Println()

	// 4. Test specific room (use first available)
	fmt.Println("=== 4. Test GetBedsByRoom Logic (First Class Room) ===")
	rows4, err := db.Query(`
		SELECT TOP 1 class_room_id
		FROM sk_bed WITH (NOLOCK)
		WHERE tgl_berakhir IS NULL
		ORDER BY class_room_id
	`)
	if err != nil {
		log.Printf("Query error: %v", err)
	} else {
		defer rows4.Close()
		
		var testRoomID string
		if rows4.Next() {
			if err := rows4.Scan(&testRoomID); err != nil {
				log.Printf("Scan error: %v", err)
			} else {
				fmt.Printf("Testing class_room_id: %s\n\n", testRoomID)
				
				// Phase 1: sk_bed defaults
				fmt.Println("  Phase 1: sk_bed defaults (creates kamar groups)")
				p1Rows, err := db.Query(`
					SELECT 
						ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key,
						id_tt_siranap,
						covid,
						ISNULL(kodekelas, '') as id_kelas,
						ISNULL(namakelas, '') as nm_kelas
					FROM sk_bed WITH (NOLOCK)
					WHERE class_room_id = @p1 AND tgl_berakhir IS NULL
				`, testRoomID)
				
				if err != nil {
					fmt.Printf("    ✗ Query error: %v\n", err)
				} else {
					defer p1Rows.Close()
					groupCount := 0
					for p1Rows.Next() {
						var kamarKey, idTT, idKelas, nmKelas string
						var covid sql.NullInt64
						if err := p1Rows.Scan(&kamarKey, &idTT, &covid, &idKelas, &nmKelas); err != nil {
							log.Printf("Scan error: %v", err)
							continue
						}
						covidVal := "0"
						if covid.Valid {
							covidVal = fmt.Sprintf("%d", covid.Int64)
						}
						fmt.Printf("    Group: %-20s id_tt=%s, covid=%s, id_kelas=%s, nm_kelas=%s\n",
							kamarKey, idTT, covidVal, idKelas, nmKelas)
						groupCount++
					}
					fmt.Printf("    ✓ Found %d kamar groups\n", groupCount)
				}
				
				fmt.Println()
				
				// Phase 2: beds data
				fmt.Println("  Phase 2: beds data (populates groups)")
				p2Rows, err := db.Query(`
					SELECT 
						bed_id,
						ISNULL(kamar, '') as kamar,
						room_id,
						id_kelas,
						nm_kelas
					FROM beds WITH (NOLOCK)
					WHERE class_room_id = @p1
					ORDER BY kamar, bed_id
				`, testRoomID)
				
				if err != nil {
					fmt.Printf("    ✗ Query error: %v\n", err)
				} else {
					defer p2Rows.Close()
					bedCount := 0
					for p2Rows.Next() {
						var bedID int
						var kamar, roomID, idKelas, nmKelas string
						if err := p2Rows.Scan(&bedID, &kamar, &roomID, &idKelas, &nmKelas); err != nil {
							log.Printf("Scan error: %v", err)
							continue
						}
						fmt.Printf("    Bed %d: kamar=%-20s room_id=%s, nm_kelas=%s\n",
							bedID, kamar, roomID, nmKelas)
						bedCount++
					}
					if bedCount == 0 {
						fmt.Println("    ⚠ No beds found - accordion will be empty!")
					} else {
						fmt.Printf("    ✓ Found %d beds\n", bedCount)
					}
				}
			}
		} else {
			fmt.Println("  ⚠ No active class rooms to test")
		}
	}
	fmt.Println()

	fmt.Println("========================================")
	fmt.Println("Diagnostic complete!")
	fmt.Println("========================================")
}

func getEnv(key, fallback string) string {
	if value := os.Getenv(key); value != "" {
		return value
	}
	return fallback
}
