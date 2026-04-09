package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type SKImportRequest struct {
	SKNo       string     `json:"sk_no"`
	TglBerlaku string     `json:"tgl_berlaku"`
	Rows       []SKBedRow `json:"rows"`
}

type SKBedRow struct {
	ClinicID        string `json:"clinic_id"`
	ClassRoomID     string `json:"class_room_id"`
	Kelas           string `json:"kelas"`
	Bed             int    `json:"bed"`
	IDTTSiranap     string `json:"id_tt_siranap"`
	RuangSiranap    string `json:"ruang_siranap"`
	KelasSiranap    string `json:"kelas_siranap"`
	Covid           int    `json:"covid"`
	Siranap         string `json:"siranap"`
	JmlRuangSiranap int    `json:"jml_ruang_siranap"`
	KodeKelas       string `json:"kodekelas"`
	NamaKelas       string `json:"namakelas"`
	NamaRuang       string `json:"namaruang"`
	Kris            string `json:"kris"`
	Kamar           string `json:"kamar"`
}

type SKRepository struct {
	db *sql.DB
}

func NewSKRepository(db *sql.DB) *SKRepository {
	return &SKRepository{db: db}
}

// BulkInsertSKBed menyimpan batch data ke dalam sk_bed dalam 1 transaksi
func (r *SKRepository) BulkInsertSKBed(ctx context.Context, req SKImportRequest) (int, error) {
	if req.SKNo == "" {
		return 0, fmt.Errorf("sk_no tidak boleh kosong")
	}
	if len(req.Rows) == 0 {
		return 0, fmt.Errorf("data rows tidak boleh kosong")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return 0, fmt.Errorf("gagal membuka transaksi: %w", err)
	}

	defer func() {
		_ = tx.Rollback()
	}()

	// Ambil max_id
	var maxID sql.NullInt64
	err = tx.QueryRowContext(ctx, "SELECT MAX(id) FROM sk_bed").Scan(&maxID)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("gagal mendapatkan max id: %w", err)
	}
	currentID := int64(0)
	if maxID.Valid {
		currentID = maxID.Int64
	}

	// Pastikan proses ini idempotent: Jika SK yang sama diinput ulang (misal: user nyimpan bertahap/iteratif), 
	// kita hapus dulu data draft/eksisting SK tersebut agar tidak terjadi duplikasi baris.
	_, err = tx.ExecContext(ctx, "DELETE FROM sk_bed WHERE sk_no = ?", req.SKNo)
	if err != nil {
		return 0, fmt.Errorf("gagal membersihkan data timpaan SK eksisting: %w", err)
	}

	// Cek apakah sk lama ada (yang aktif saat ini)
	var oldSKNo sql.NullString
	err = tx.QueryRowContext(ctx, "SELECT DISTINCT sk_no FROM sk_bed WHERE tgl_berakhir IS NULL").Scan(&oldSKNo)
	if err != nil && err != sql.ErrNoRows {
		return 0, fmt.Errorf("gagal mencari SK aktif lama: %w", err)
	}

	// Jika ada SK yang sedang aktif, DAN nomornya BERBEDA dengan yang sedang diinputkan, 
	// maka SK Lama ini kita pensiunkan (tgl_berakhir = H-1).
	// Jika sama (oldSKNo == req.SKNo), artinya user sedang memperbarui/menambah baris ke SK aktif saat ini.
	if oldSKNo.Valid && oldSKNo.String != "" && oldSKNo.String != req.SKNo {
		// Update tgl_berakhir SK Lama menjadi H-1 dari tgl_berlaku SK Baru
		_, err = tx.ExecContext(ctx,
			"UPDATE sk_bed SET tgl_berakhir = DATEADD(day, -1, CAST(? AS DATE)) WHERE sk_no = ?",
			req.TglBerlaku, oldSKNo.String,
		)
		if err != nil {
			return 0, fmt.Errorf("gagal menonaktifkan SK lama (%s): %w", oldSKNo.String, err)
		}
		log.Printf("[INFO] SK lama %s dinonaktifkan", oldSKNo.String)
	}

	// Loop dan insert
	insertedCount := 0
	insertQuery := `
		INSERT INTO sk_bed (
			id, clinic_id, class_room_id, kelas, bed, sk_no, 
			tgl_berlaku, tgl_berakhir, id_tt_siranap, ruang_siranap,
			kelas_siranap, covid, siranap, jml_ruang_siranap, 
			kodekelas, namakelas, namaruang, kris, kamar
		) VALUES (
			?, ?, ?, ?, ?, ?, 
			?, NULL, ?, ?,
			?, ?, ?, ?, 
			?, ?, ?, ?, ?
		)`

	stmt, err := tx.PrepareContext(ctx, insertQuery)
	if err != nil {
		return 0, fmt.Errorf("gagal menyiapkan query insert: %w", err)
	}
	defer stmt.Close()

	for _, row := range req.Rows {
		currentID++
		_, err := stmt.ExecContext(ctx,
			currentID,
			row.ClinicID,
			row.ClassRoomID,
			row.Kelas,
			row.Bed,
			req.SKNo,
			req.TglBerlaku,
			row.IDTTSiranap,
			row.RuangSiranap,
			row.KelasSiranap,
			row.Covid,
			row.Siranap,
			row.JmlRuangSiranap,
			row.KodeKelas,
			row.NamaKelas,
			row.NamaRuang,
			row.Kris,
			row.Kamar,
		)
		if err != nil {
			return 0, fmt.Errorf("gagal insert baris ke-%d: %w", insertedCount+1, err)
		}
		insertedCount++
	}

	if err := tx.Commit(); err != nil {
		return 0, fmt.Errorf("gagal commit transaksi: %w", err)
	}

	return insertedCount, nil
}

// GetSKList mengambil daftar sk_no unik dari tabel sk_bed.
func (r *SKRepository) GetSKList(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT sk_no FROM sk_bed ORDER BY sk_no DESC`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal query daftar SK: %w", err)
	}
	defer rows.Close()

	var list []string
	for rows.Next() {
		var sk string
		if err := rows.Scan(&sk); err != nil {
			return nil, fmt.Errorf("gagal scan sk_no: %w", err)
		}
		list = append(list, sk)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi rows daftar SK gagal: %w", err)
	}
	return list, nil
}

// GetSKDetail mengambil semua baris data untuk sebuah sk_no tertentu.
func (r *SKRepository) GetSKDetail(ctx context.Context, skNo string) ([]SKBedRow, error) {
	query := `
		SELECT 
			clinic_id, class_room_id, kelas, bed, id_tt_siranap, 
			ruang_siranap, kelas_siranap, covid, siranap, jml_ruang_siranap, 
			kodekelas, namakelas, namaruang, kris, kamar
		FROM sk_bed
		WHERE sk_no = ?
		ORDER BY id ASC
	`
	rows, err := r.db.QueryContext(ctx, query, skNo)
	if err != nil {
		return nil, fmt.Errorf("gagal query detail SK: %w", err)
	}
	defer rows.Close()

	var result []SKBedRow
	for rows.Next() {
		var row SKBedRow
		var clinicID, classRoomID, kelas, idTTSiranap, ruangSiranap, kelasSiranap, siranap, kodekelas, namakelas, namaruang, kris, kamar sql.NullString
		var bed, covid, jmlRuang sql.NullInt64

		err := rows.Scan(
			&clinicID, &classRoomID, &kelas, &bed, &idTTSiranap,
			&ruangSiranap, &kelasSiranap, &covid, &siranap, &jmlRuang,
			&kodekelas, &namakelas, &namaruang, &kris, &kamar,
		)
		if err != nil {
			return nil, fmt.Errorf("gagal scan baris detail SK: %w", err)
		}

		if clinicID.Valid { row.ClinicID = clinicID.String }
		if classRoomID.Valid { row.ClassRoomID = classRoomID.String }
		if kelas.Valid { row.Kelas = kelas.String }
		if bed.Valid { row.Bed = int(bed.Int64) }
		if idTTSiranap.Valid { row.IDTTSiranap = idTTSiranap.String }
		if ruangSiranap.Valid { row.RuangSiranap = ruangSiranap.String }
		if kelasSiranap.Valid { row.KelasSiranap = kelasSiranap.String }
		if covid.Valid { row.Covid = int(covid.Int64) }
		if siranap.Valid { row.Siranap = siranap.String }
		if jmlRuang.Valid { row.JmlRuangSiranap = int(jmlRuang.Int64) }
		if kodekelas.Valid { row.KodeKelas = kodekelas.String }
		if namakelas.Valid { row.NamaKelas = namakelas.String }
		if namaruang.Valid { row.NamaRuang = namaruang.String }
		if kris.Valid { row.Kris = kris.String }
		if kamar.Valid { row.Kamar = kamar.String }

		result = append(result, row)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi rows detail SK gagal: %w", err)
	}
	return result, nil
}

