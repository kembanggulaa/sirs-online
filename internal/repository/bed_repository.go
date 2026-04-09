package repository

import (
	"database/sql"
	"fmt"
)

// BedSiranap merepresentasikan satu baris data ketersediaan tempat tidur
// yang akan dikirim ke API RS Online Kemenkes.
type BedSiranap struct {
	IDTTSiranap  string `json:"id_tt_siranap"`
	ClassRoomID  string `json:"class_room_id"`
	Siranap      string `json:"ruang"`
	JmlRuang     int    `json:"jumlah_ruang"`
	Kelas        string `json:"kelas"`
	Kamar        string `json:"kamar"`
	KelasSiranap string `json:"kelas_siranap"`
	Jumlah       int    `json:"jumlah"`
	Terisi       int    `json:"terpakai"`
	Status       int    `json:"terpakai_suspek"`
	Konfirmasi   int    `json:"terpakai_konfirmasi"`
	Antrian      int    `json:"antrian"`
	Covid        int    `json:"covid"`
	Prepare      int    `json:"prepare"`
	PreparePlan  int    `json:"prepare_plan"`
}

// BedRepository mengelola koneksi ke database SIMRS
type BedRepository struct {
	db *sql.DB
}

// New membuat instance BedRepository baru
func New(db *sql.DB) *BedRepository {
	return &BedRepository{db: db}
}

// GetActiveSKNo mengambil sk_no yang masih aktif (tgl_berakhir IS NULL).
// Query 0 — dijalankan setiap kali sebelum worker memproses data.
func (r *BedRepository) GetActiveSKNo() (string, error) {
	query := `SELECT DISTINCT sk_no FROM sk_bed WHERE tgl_berakhir IS NULL`

	rows, err := r.db.Query(query)
	if err != nil {
		return "", fmt.Errorf("query sk aktif gagal: %w", err)
	}
	defer rows.Close()

	var skNo string
	if rows.Next() {
		if err := rows.Scan(&skNo); err != nil {
			return "", fmt.Errorf("scan sk_no gagal: %w", err)
		}
	}

	// Cek error yang terjadi selama iterasi
	if err := rows.Err(); err != nil {
		return "", fmt.Errorf("iterasi rows sk aktif gagal: %w", err)
	}

	if skNo == "" {
		return "", fmt.Errorf("tidak ada SK aktif ditemukan di tabel sk_bed")
	}

	return skNo, nil
}

// GetBedAvailability mengambil data ketersediaan tempat tidur menggunakan
// satu transaksi SQL Server (karena temp table #temp_ranap hanya hidup dalam satu sesi).
//
// Urutan query:
//  1. Query 1 — CREATE #temp_ranap (pasien rawat inap aktif)
//  2. Query 2 — SELECT dengan LEFT JOIN ke #temp_ranap
func (r *BedRepository) GetBedAvailability(skNo string) ([]BedSiranap, error) {
	// Buka transaksi — wajib agar #temp_ranap hidup sepanjang kedua query
	tx, err := r.db.Begin()
	if err != nil {
		return nil, fmt.Errorf("gagal membuka transaksi: %w", err)
	}
	defer func() {
		// Rollback jika tidak di-commit (cleanup temp table juga ikut bersama sesi)
		_ = tx.Rollback()
	}()

	// ─── Query 1: Buat temp table pasien rawat inap aktif ───────────────────
	// Menggunakan named parameter @sk_no via mssql.Named untuk mencegah SQL injection.
	// Catatan: go-mssqldb mendukung named param dalam subquery IN clause.
	query1 := `
		SELECT
			no_registration,
			class_room_id,
			BED_ID,
			keluar_id,
			(SELECT CONCAT(class_room_id, kamar)
			 FROM beds b
			 WHERE b.class_room_id = pv.CLASS_ROOM_ID
			   AND b.bed_id = pv.bed_id) AS kamar
		INTO #temp_ranap
		FROM pasien_visitation pv
		WHERE no_registration <> ''
		  AND class_room_id IS NOT NULL
		  AND (pv.keluar_id = 0 OR pv.keluar_id = 33)
		  AND class_room_id IN (
			SELECT DISTINCT class_room_id
			FROM sk_bed
			WHERE sk_no = @sk_no
			  AND tgl_berakhir IS NULL
			  AND class_room_id <> 'NI.BX'
		  )
		ORDER BY kamar`

	if _, err := tx.Exec(query1, sql.Named("sk_no", skNo)); err != nil {
		return nil, fmt.Errorf("query 1 (temp table) gagal: %w", err)
	}

	// ─── Query 2: Ambil ketersediaan bed dengan LEFT JOIN ke #temp_ranap ────
	query2 := `
		SELECT
			sk.id_tt_siranap,
			sk.class_room_id,
			IIF(sk.kamar IS NULL, sk.ruang_siranap, CONCAT(sk.ruang_siranap, '-', sk.kamar)) AS siranap,
			sk.jml_ruang_siranap,
			sk.kelas_siranap AS kelas,
			CONCAT(sk.class_room_id, sk.kamar) AS kamar,
			sk.kelas_siranap,
			SUM(sk.bed) AS jumlah,
			sk.covid,
			sc.status,
			sc.konfirmasi,
			sc.antrian,
			ISNULL(t.terisi, 0) AS terisi
		FROM sk_bed sk
			INNER JOIN status_covid sc ON sc.id_tt = sk.id_tt_siranap
			LEFT JOIN (
				SELECT kamar, COUNT(*) AS terisi
				FROM #temp_ranap
				GROUP BY kamar
			) t ON t.kamar = CONCAT(sk.class_room_id, sk.kamar)
		WHERE sk.sk_no = @sk_no
		  AND sk.tgl_berakhir IS NULL
		  AND sk.class_room_id <> 'NI.BX'
		GROUP BY
			sk.id_tt_siranap, sk.class_room_id, sk.siranap, sk.jml_ruang_siranap,
			sk.kamar, sk.kelas_siranap, sk.ruang_siranap, sk.covid,
			sc.status, sc.konfirmasi, sc.antrian, t.terisi
		ORDER BY sk.siranap, sk.ruang_siranap`

	rows, err := tx.Query(query2, sql.Named("sk_no", skNo))
	if err != nil {
		return nil, fmt.Errorf("query 2 (ketersediaan bed) gagal: %w", err)
	}
	defer rows.Close()

	var beds []BedSiranap
	for rows.Next() {
		var b BedSiranap
		err := rows.Scan(
			&b.IDTTSiranap,
			&b.ClassRoomID,
			&b.Siranap,
			&b.JmlRuang,
			&b.Kelas,
			&b.Kamar,
			&b.KelasSiranap,
			&b.Jumlah,
			&b.Covid,
			&b.Status,
			&b.Konfirmasi,
			&b.Antrian,
			&b.Terisi,
		)
		if err != nil {
			return nil, fmt.Errorf("scan baris bed gagal: %w", err)
		}
		beds = append(beds, b)
	}

	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi rows gagal: %w", err)
	}

	// Commit transaksi — temp table otomatis hilang setelah sesi selesai
	if err := tx.Commit(); err != nil {
		return nil, fmt.Errorf("commit transaksi gagal: %w", err)
	}

	return beds, nil
}
