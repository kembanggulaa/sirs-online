package repository

import (
	"context"
	"database/sql"
	"fmt"
	"log"
)

type BedsUpsertRequest struct {
	ClassRoomID string   `json:"class_room_id"`
	Rows        []BedRow `json:"rows"`
}

type BedRow struct {
	BedID            int    `json:"bed_id"`
	Kamar            string `json:"kamar"`
	RoomID           string `json:"room_id"`
	IDKelas          string `json:"id_kelas"`
	NmKelas          string `json:"nm_kelas"`
	IDPerawatan      string `json:"id_perawatan"`
	NmPerawatan      string `json:"nm_perawatan"`
	IDTTSiranap      string `json:"id_tt_siranap"`
	IDSiranap        string `json:"id_siranap"`
	DeskripsiSiranap string `json:"deskripsi_siranap"`
	Covid            string `json:"covid"`
}

type BedsKamarGroup struct {
	Kamar    string            `json:"kamar"`
	Defaults map[string]string `json:"defaults"`
	Rows     []BedRow          `json:"rows"`
}

type BedsRoomResult struct {
	Mode   string           `json:"mode"`
	Kamars []BedsKamarGroup `json:"kamars"`
}

type UpsertResult struct {
	Saved    int `json:"saved"`
	Inserted int `json:"inserted"`
	Updated  int `json:"updated"`
	Deleted  int `json:"deleted"`
}

type BedsRepository struct {
	db *sql.DB
}

func NewBedsRepository(db *sql.DB) *BedsRepository {
	return &BedsRepository{db: db}
}

// GetDistinctClassRooms mengambil daftar class_room_id unik dari sk_bed aktif.
func (r *BedsRepository) GetDistinctClassRooms(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT class_room_id FROM sk_bed WHERE tgl_berakhir IS NULL ORDER BY class_room_id`
	rows, err := r.db.QueryContext(ctx, query)
	if err != nil {
		return nil, fmt.Errorf("gagal query class_room_id: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var id sql.NullString
		if err := rows.Scan(&id); err != nil {
			return nil, fmt.Errorf("gagal scan class_room_id: %w", err)
		}
		if id.Valid && id.String != "" {
			result = append(result, id.String)
		}
	}
	return result, nil
}

// GetKamarByClassRoom mengambil daftar kamar berdasarkan class_room_id dari sk_bed.
func (r *BedsRepository) GetKamarByClassRoom(ctx context.Context, classRoomID string) ([]string, error) {
	query := `SELECT DISTINCT kamar FROM sk_bed WHERE class_room_id = ? AND tgl_berakhir IS NULL ORDER BY kamar`
	rows, err := r.db.QueryContext(ctx, query, classRoomID)
	if err != nil {
		return nil, fmt.Errorf("gagal query kamar: %w", err)
	}
	defer rows.Close()

	var result []string
	for rows.Next() {
		var k sql.NullString
		if err := rows.Scan(&k); err != nil {
			return nil, fmt.Errorf("gagal scan kamar: %w", err)
		}
		if k.Valid && k.String != "" {
			result = append(result, k.String)
		}
	}
	// Selalu kembalikan array (jangan nil)
	if result == nil {
		result = []string{}
	}
	return result, nil
}

// GetBedsByRoom mengambil semua data beds untuk class_room_id dan digrup ke dalam accordion (BedsKamarGroup).
func (r *BedsRepository) GetBedsByRoom(ctx context.Context, classRoomID string) (BedsRoomResult, error) {
	result := BedsRoomResult{
		Mode:   "new",
		Kamars: []BedsKamarGroup{},
	}

	kamarsMap := make(map[string]*BedsKamarGroup)
	kamarDefaults := make(map[string]map[string]string)
	var kamarOrder []string

	// 1. Fetch defaults dari sk_bed berdasarkan kamar
	queryDefaults := `
		SELECT ISNULL(kamar, ''), id_tt_siranap, covid 
		FROM sk_bed 
		WHERE class_room_id = ? AND tgl_berakhir IS NULL
	`
	rowsDef, err := r.db.QueryContext(ctx, queryDefaults, classRoomID)
	if err == nil {
		for rowsDef.Next() {
			var k, idTT sql.NullString
			var cov sql.NullInt64
			if err := rowsDef.Scan(&k, &idTT, &cov); err == nil {
				kName := k.String
				if _, exists := kamarDefaults[kName]; !exists {
					kamarDefaults[kName] = map[string]string{"id_tt_siranap": "", "covid": "0"}
				}
				if idTT.Valid { kamarDefaults[kName]["id_tt_siranap"] = idTT.String }
				if cov.Valid { kamarDefaults[kName]["covid"] = fmt.Sprintf("%d", cov.Int64) }
			}
		}
		rowsDef.Close()
	}

	// 2. Fetch existing beds (all kamars)
	queryBeds := `
		SELECT bed_id, ISNULL(kamar, ''), room_id, id_kelas, nm_kelas, id_perawatan, nm_perawatan, 
		       id_tt_siranap, id_siranap, deskripsi_siranap, covid
		FROM beds
		WHERE class_room_id = ?
		ORDER BY kamar, bed_id
	`
	rows, err := r.db.QueryContext(ctx, queryBeds, classRoomID)
	if err == nil {
		for rows.Next() {
			var b BedRow
			var bRoomID, bIDKelas, bNmKelas, bIDPer, bNmPer, bIDTT, bIDSir, bDescSir, bKamar sql.NullString
			var bBedID, bCovid sql.NullInt64

			if err := rows.Scan(
				&bBedID, &bKamar, &bRoomID, &bIDKelas, &bNmKelas, &bIDPer, &bNmPer,
				&bIDTT, &bIDSir, &bDescSir, &bCovid,
			); err != nil {
				continue
			}

			kName := ""
			if bKamar.Valid { kName = bKamar.String }

			if bBedID.Valid { b.BedID = int(bBedID.Int64) }
			b.Kamar = kName
			if bRoomID.Valid { b.RoomID = bRoomID.String }
			if bIDKelas.Valid { b.IDKelas = bIDKelas.String }
			if bNmKelas.Valid { b.NmKelas = bNmKelas.String }
			if bIDPer.Valid { b.IDPerawatan = bIDPer.String }
			if bNmPer.Valid { b.NmPerawatan = bNmPer.String }
			if bIDTT.Valid { b.IDTTSiranap = bIDTT.String }
			if bIDSir.Valid { b.IDSiranap = bIDSir.String }
			if bDescSir.Valid { b.DeskripsiSiranap = bDescSir.String }
			if bCovid.Valid { b.Covid = fmt.Sprintf("%d", bCovid.Int64) }

			if _, exists := kamarsMap[kName]; !exists {
				defs := map[string]string{"id_tt_siranap": "", "covid": "0"}
				if d, ok := kamarDefaults[kName]; ok {
					defs = d
				} else if d, ok := kamarDefaults[""]; ok {
					defs = d
				}

				kamarsMap[kName] = &BedsKamarGroup{
					Kamar:    kName,
					Defaults: defs,
					Rows:     []BedRow{},
				}
				kamarOrder = append(kamarOrder, kName)
			}

			kamarsMap[kName].Rows = append(kamarsMap[kName].Rows, b)
			result.Mode = "edit"
		}
		rows.Close()
	}

	for _, kName := range kamarOrder {
		result.Kamars = append(result.Kamars, *kamarsMap[kName])
	}

	return result, nil
}

// UpsertBeds melakukan INSERT/UPDATE beds dalam satu transaksi.
func (r *BedsRepository) UpsertBeds(ctx context.Context, req BedsUpsertRequest) (UpsertResult, error) {
	var res UpsertResult
	if req.ClassRoomID == "" {
		return res, fmt.Errorf("class_room_id wajib diisi")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("gagal membuka transaksi: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Ambil existing bed_id list untuk keseluruhan class_room_id
	queryExisting := `SELECT bed_id FROM beds WHERE class_room_id = ?`
	rows, err := tx.QueryContext(ctx, queryExisting, req.ClassRoomID)
	
	existingMap := make(map[int]bool)
	if err == nil {
		for rows.Next() {
			var bid int
			if err := rows.Scan(&bid); err == nil {
				existingMap[bid] = true
			}
		}
		rows.Close()
	} else if err != sql.ErrNoRows {
		log.Printf("[WARN] Cek existing beds error: %v", err)
	}

	orgUnitCode := "3404011"

	insertQuery := `
		INSERT INTO beds (
			bed_id, class_room_id, org_unit_code, room_id, 
			id_kelas, nm_kelas, id_perawatan, nm_perawatan, 
			id_tt_siranap, id_siranap, deskripsi_siranap, covid, kamar
		) VALUES (?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?, ?)
	`
	updateQuery := `
		UPDATE beds SET 
			org_unit_code = ?, room_id = ?, id_kelas = ?, nm_kelas = ?, 
			id_perawatan = ?, nm_perawatan = ?, id_tt_siranap = ?, 
			id_siranap = ?, deskripsi_siranap = ?, covid = ?, kamar = ?
		WHERE bed_id = ? AND class_room_id = ?
	`

	stmtInsert, errInsert := tx.PrepareContext(ctx, insertQuery)
	stmtUpdate, errUpdate := tx.PrepareContext(ctx, updateQuery)

	for i, row := range req.Rows {
		if row.BedID == 0 {
			return res, fmt.Errorf("ada baris (%d) tidak memiliki bed_id", i+1)
		}
		if row.IDKelas == "" || row.NmKelas == "" {
			return res, fmt.Errorf("baris bed_id %d tidak memiliki id_kelas/nm_kelas", row.BedID)
		}

		if existingMap[row.BedID] {
			// UPDATE
			if errUpdate != nil {
				return res, fmt.Errorf("gagal menyiapkan query update: %w", errUpdate)
			}
			_, err = stmtUpdate.ExecContext(ctx,
				orgUnitCode, row.RoomID, row.IDKelas, row.NmKelas,
				row.IDPerawatan, row.NmPerawatan, row.IDTTSiranap,
				row.IDSiranap, row.DeskripsiSiranap, row.Covid, row.Kamar,
				row.BedID, req.ClassRoomID,
			)
			if err != nil {
				return res, fmt.Errorf("gagal update bed_id %d: %w", row.BedID, err)
			}
			res.Updated++
			// Hapus dari map existingMap untuk menandai bahwa ini telah diproses
			delete(existingMap, row.BedID)
		} else {
			// INSERT
			if errInsert != nil {
				return res, fmt.Errorf("gagal menyiapkan query insert: %w", errInsert)
			}
			_, err = stmtInsert.ExecContext(ctx,
				row.BedID, req.ClassRoomID, orgUnitCode, row.RoomID,
				row.IDKelas, row.NmKelas, row.IDPerawatan, row.NmPerawatan,
				row.IDTTSiranap, row.IDSiranap, row.DeskripsiSiranap, row.Covid, row.Kamar,
			)
			if err != nil {
				return res, fmt.Errorf("gagal insert bed_id %d: %w", row.BedID, err)
			}
			res.Inserted++
		}
	}

	// Sisa data yang ada di existingMap tapi tidak ada di payload req.Rows berarti telah Dihapus oleh user
	if len(existingMap) > 0 {
		deleteQuery := `DELETE FROM beds WHERE bed_id = ? AND class_room_id = ?`
		stmtDelete, errDelete := tx.PrepareContext(ctx, deleteQuery)
		if errDelete == nil {
			for bedID := range existingMap {
				_, err = stmtDelete.ExecContext(ctx, bedID, req.ClassRoomID)
				if err == nil {
					res.Deleted++
				}
			}
			stmtDelete.Close()
		}
	}

	if stmtInsert != nil { stmtInsert.Close() }
	if stmtUpdate != nil { stmtUpdate.Close() }

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("gagal commit transaksi beds: %w", err)
	}

	res.Saved = res.Inserted + res.Updated
	return res, nil
}
