package repository

import (
	"context"
	"database/sql"
	"fmt"
)

type BedsUpsertRequest struct {
	ClassRoomID string   `json:"class_room_id"`
	OrgUnitCode string   `json:"org_unit_code,omitempty"` // opsional — fallback ke config
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
	db          *sql.DB
	orgUnitCode string // kode org unit dari konfigurasi
}

// NewBedsRepository membuat BedsRepository baru.
// orgUnitCode diambil dari config.Operational.OrgUnitCode agar tidak hardcode di repository.
func NewBedsRepository(db *sql.DB, orgUnitCode string) *BedsRepository {
	return &BedsRepository{db: db, orgUnitCode: orgUnitCode}
}

// GetDistinctClassRooms mengambil daftar class_room_id unik dari sk_bed aktif.
func (r *BedsRepository) GetDistinctClassRooms(ctx context.Context) ([]string, error) {
	query := `SELECT DISTINCT class_room_id FROM sk_bed WITH (NOLOCK) WHERE tgl_berakhir IS NULL ORDER BY class_room_id`
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi rows class_room_id gagal: %w", err)
	}
	return result, nil
}

// GetKamarByClassRoom mengambil daftar kamar berdasarkan class_room_id dari sk_bed.
func (r *BedsRepository) GetKamarByClassRoom(ctx context.Context, classRoomID string) ([]string, error) {
	query := `SELECT DISTINCT kamar FROM sk_bed WITH (NOLOCK) WHERE class_room_id = @p1 AND tgl_berakhir IS NULL ORDER BY kamar`
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
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterasi rows kamar gagal: %w", err)
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
		FROM sk_bed WITH (NOLOCK)
		WHERE class_room_id = @p1 AND tgl_berakhir IS NULL
	`
	rowsDef, err := r.db.QueryContext(ctx, queryDefaults, classRoomID)
	if err != nil {
		return result, fmt.Errorf("gagal query defaults sk_bed: %w", err)
	}
	defer rowsDef.Close()

	for rowsDef.Next() {
		var k, idTT sql.NullString
		var cov sql.NullInt64
		if err := rowsDef.Scan(&k, &idTT, &cov); err != nil {
			return result, fmt.Errorf("gagal scan defaults sk_bed: %w", err)
		}
		kName := k.String
		if _, exists := kamarDefaults[kName]; !exists {
			kamarDefaults[kName] = map[string]string{"id_tt_siranap": "", "covid": "0"}

			// Populate initial kamarsMap so the user immediately sees all kamars defined in sk_bed
			kamarsMap[kName] = &BedsKamarGroup{
				Kamar:    kName,
				Defaults: kamarDefaults[kName],
				Rows:     []BedRow{},
			}
			kamarOrder = append(kamarOrder, kName)
		}
		if idTT.Valid {
			kamarDefaults[kName]["id_tt_siranap"] = idTT.String
			kamarsMap[kName].Defaults["id_tt_siranap"] = idTT.String
		}
		if cov.Valid {
			strCov := fmt.Sprintf("%d", cov.Int64)
			kamarDefaults[kName]["covid"] = strCov
			kamarsMap[kName].Defaults["covid"] = strCov
		}
	}
	if err := rowsDef.Err(); err != nil {
		return result, fmt.Errorf("iterasi rows defaults gagal: %w", err)
	}

	// 2. Fetch existing beds (all kamars)
	queryBeds := `
		SELECT bed_id, ISNULL(kamar, ''), room_id, id_kelas, nm_kelas, id_perawatan, nm_perawatan, 
		       id_tt_siranap, id_siranap, deskripsi_siranap, covid
		FROM beds WITH (NOLOCK)
		WHERE class_room_id = @p1
		ORDER BY kamar, bed_id
	`
	rows, err := r.db.QueryContext(ctx, queryBeds, classRoomID)
	if err != nil {
		return result, fmt.Errorf("gagal query beds: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		var b BedRow
		var bRoomID, bIDKelas, bNmKelas, bIDPer, bNmPer, bIDTT, bIDSir, bDescSir, bKamar sql.NullString
		var bBedID, bCovid sql.NullInt64

		if err := rows.Scan(
			&bBedID, &bKamar, &bRoomID, &bIDKelas, &bNmKelas, &bIDPer, &bNmPer,
			&bIDTT, &bIDSir, &bDescSir, &bCovid,
		); err != nil {
			return result, fmt.Errorf("gagal scan baris beds: %w", err)
		}

		kName := ""
		if bKamar.Valid {
			kName = bKamar.String
		}

		if bBedID.Valid {
			b.BedID = int(bBedID.Int64)
		}
		b.Kamar = kName
		if bRoomID.Valid {
			b.RoomID = bRoomID.String
		}
		if bIDKelas.Valid {
			b.IDKelas = bIDKelas.String
		}
		if bNmKelas.Valid {
			b.NmKelas = bNmKelas.String
		}
		if bIDPer.Valid {
			b.IDPerawatan = bIDPer.String
		}
		if bNmPer.Valid {
			b.NmPerawatan = bNmPer.String
		}
		if bIDTT.Valid {
			b.IDTTSiranap = bIDTT.String
		}
		if bIDSir.Valid {
			b.IDSiranap = bIDSir.String
		}
		if bDescSir.Valid {
			b.DeskripsiSiranap = bDescSir.String
		}
		if bCovid.Valid {
			b.Covid = fmt.Sprintf("%d", bCovid.Int64)
		}

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
	if err := rows.Err(); err != nil {
		return result, fmt.Errorf("iterasi rows beds gagal: %w", err)
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

	// Gunakan org_unit_code dari request jika ada, fallback ke config
	orgUnitCode := req.OrgUnitCode
	if orgUnitCode == "" {
		orgUnitCode = r.orgUnitCode
	}
	if orgUnitCode == "" {
		return res, fmt.Errorf("org_unit_code tidak dikonfigurasi")
	}

	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return res, fmt.Errorf("gagal membuka transaksi: %w", err)
	}
	defer func() {
		_ = tx.Rollback()
	}()

	// Ambil existing bed_id list untuk keseluruhan class_room_id
	queryExisting := `SELECT bed_id FROM beds WHERE class_room_id = @p1`
	rows, err := tx.QueryContext(ctx, queryExisting, req.ClassRoomID)
	if err != nil {
		return res, fmt.Errorf("gagal query existing beds: %w", err)
	}

	existingMap := make(map[int]bool)
	for rows.Next() {
		var bid int
		if err := rows.Scan(&bid); err != nil {
			rows.Close()
			return res, fmt.Errorf("gagal scan existing bed_id: %w", err)
		}
		existingMap[bid] = true
	}
	rows.Close()
	if err := rows.Err(); err != nil {
		return res, fmt.Errorf("iterasi rows existing beds gagal: %w", err)
	}

	insertQuery := `
		INSERT INTO beds (
			bed_id, class_room_id, org_unit_code, room_id, 
			id_kelas, nm_kelas, id_perawatan, nm_perawatan, 
			id_tt_siranap, id_siranap, deskripsi_siranap, covid, kamar
		) VALUES (@p1, @p2, @p3, @p4, @p5, @p6, @p7, @p8, @p9, @p10, @p11, @p12, @p13)
	`
	updateQuery := `
		UPDATE beds SET 
			org_unit_code = @p1, room_id = @p2, id_kelas = @p3, nm_kelas = @p4, 
			id_perawatan = @p5, nm_perawatan = @p6, id_tt_siranap = @p7, 
			id_siranap = @p8, deskripsi_siranap = @p9, covid = @p10, kamar = @p11
		WHERE bed_id = @p12 AND class_room_id = @p13
	`

	stmtInsert, errInsert := tx.PrepareContext(ctx, insertQuery)
	stmtUpdate, errUpdate := tx.PrepareContext(ctx, updateQuery)
	defer func() {
		if stmtInsert != nil {
			stmtInsert.Close()
		}
		if stmtUpdate != nil {
			stmtUpdate.Close()
		}
	}()

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
		deleteQuery := `DELETE FROM beds WHERE bed_id = @p1 AND class_room_id = @p2`
		stmtDelete, err := tx.PrepareContext(ctx, deleteQuery)
		if err != nil {
			return res, fmt.Errorf("gagal menyiapkan query delete: %w", err)
		}
		defer stmtDelete.Close()

		for bedID := range existingMap {
			if _, err = stmtDelete.ExecContext(ctx, bedID, req.ClassRoomID); err != nil {
				return res, fmt.Errorf("gagal delete bed_id %d: %w", bedID, err)
			}
			res.Deleted++
		}
	}

	if err := tx.Commit(); err != nil {
		return res, fmt.Errorf("gagal commit transaksi beds: %w", err)
	}

	res.Saved = res.Inserted + res.Updated
	return res, nil
}
