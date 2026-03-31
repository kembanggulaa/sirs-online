# Arsitektur & Task List — Fitur Input Data Beds
## Sistem Bridging RS Online V3 — RSUD Sleman

> Fitur tambahan untuk menginputkan data tempat tidur fisik (beds) ke tabel `beds` berdasarkan `class_room_id` yang sudah diinputkan melalui fitur Manajemen SK (Tab 5).

---

## 1. Ringkasan Fitur

Fitur **Manajemen Beds** memungkinkan admin IT untuk memasukkan dan mengedit data bed fisik per ruangan ke tabel `beds` melalui antarmuka spreadsheet di browser. Alur terdiri dari:

1. **Selector** → admin pilih `class_room_id` dari dropdown (sumber: `sk_bed`), lalu pilih `kamar`
2. **Fetch Data** → sistem cek apakah data beds sudah ada untuk kombinasi tersebut
3. **Mode Edit / Mode Input Baru** → tabel tampil dengan data existing (jika ada) atau kosong
4. **Upsert** → INSERT baris baru + UPDATE baris existing dalam satu transaksi

---

## 2. Posisi di Dashboard

Fitur ini ditempatkan sebagai **Tab 6 — Manajemen Beds**, tab baru setelah Tab 5 (Manajemen SK).

```
[ Tab 1: Info Ruang ] [ Tab 2: Master Referensi ] [ Tab 3: Manajemen TT ] [ Tab 4: Operasional ] [ Tab 5: Manajemen SK ] [ Tab 6: Manajemen Beds ✨ ]
```

---

## 3. Alur Kerja (Flow)

```
┌──────────────────────────────────────────────────────────────────────┐
│                       TAB 6 — Manajemen Beds                        │
│                                                                      │
│  HEADER SELECTOR                                                     │
│  ┌──────────────────────────────────────┐                            │
│  │ class_room_id : [dropdown ▼]         │  ← DISTINCT dari sk_bed   │
│  │ kamar         : [dropdown ▼]         │  ← difilter by class_room │
│  │                                      │                            │
│  │         [ Tampilkan Data ]           │                            │
│  └──────────────────────────────────────┘                            │
│              │                                                       │
│              ▼ GET /api/beds/by-room?class_room_id=X&kamar=Y        │
│              │                                                       │
│         ┌────┴────┐                                                  │
│         │         │                                                  │
│       Ada data  Kosong                                               │
│         │         │                                                  │
│         ▼         ▼                                                  │
│   🟡 MODE EDIT  🟢 MODE INPUT BARU                                   │
│   (baris lama   (tabel kosong,                                       │
│   ditampilkan,  siap diisi)                                          │
│   bisa diubah                                                        │
│   + tambah)                                                          │
│         │         │                                                  │
│         └────┬────┘                                                  │
│              ▼                                                       │
│   Tabel Spreadsheet 12 kolom (scrollable horizontal)                 │
│   + Tambah Baris  + Hapus per baris                                  │
│              │                                                       │
│              ▼ klik "Simpan"                                         │
│   POST /api/beds/upsert                                              │
│   (INSERT baris baru, UPDATE baris existing, dalam 1 transaksi)      │
│              │                                                       │
│              ▼                                                       │
│   ✅ X baris disimpan (Y inserted, Z updated)                        │
│   ❌ Gagal — rollback, tidak ada data tersimpan                      │
└──────────────────────────────────────────────────────────────────────┘
```

---

## 4. Struktur Kolom Tabel Input

### 4a. Header Selector — dipilih sekali, berlaku untuk semua baris

| Field | Input | Sumber Data |
|---|---|---|
| `class_room_id` | dropdown | `SELECT DISTINCT class_room_id FROM sk_bed WHERE tgl_berakhir IS NULL` |
| `kamar` | dropdown | `SELECT DISTINCT kamar FROM sk_bed WHERE class_room_id = ?` — bisa berisi nilai kosong jika memang tidak ada kamar di `sk_bed` |

### 4b. Tabel Spreadsheet — diisi per baris

Urutan kolom mengikuti struktur tabel `beds`:

| # | Kolom DB | Input Type | Wajib | Sumber Default |
|---|---|---|---|---|
| 1 | `bed_id` | number | ✅ | Manual admin (representasi nomor bed) |
| 2 | `class_room_id` | — | ✅ | Auto dari selector, **read-only** |
| 3 | `org_unit_code` | — | — | **Auto inject backend: `3404011`** (tidak tampil di tabel) |
| 4 | `room_id` | text | ❌ | Manual, boleh kosong |
| 5 | `id_kelas` | text | ✅ | Manual admin |
| 6 | `nm_kelas` | text | ✅ | Manual admin |
| 7 | `id_perawatan` | text | ❌ | Manual, boleh kosong |
| 8 | `nm_perawatan` | text | ❌ | Manual, boleh kosong |
| 9 | `id_tt_siranap` | text | ❌ | Auto-populate dari `sk_bed`, **bisa diedit** |
| 10 | `id_siranap` | text | ❌ | Manual, boleh kosong |
| 11 | `deskripsi_siranap` | text | ❌ | Manual, boleh kosong |
| 12 | `covid` | toggle (0/1) | ❌ | Auto-populate dari `sk_bed`, **bisa diedit** |
| 13 | `kamar` | — | ❌ | Auto dari selector, **read-only** |

**Kolom auto (tidak tampil di tabel):** `org_unit_code` (selalu `3404011`), `class_room_id` (dari selector), `kamar` (dari selector) — ketiganya di-inject oleh backend saat upsert.

**Total kolom yang diisi/tampil di tabel: 10 kolom** (`bed_id`, `room_id`, `id_kelas`, `nm_kelas`, `id_perawatan`, `nm_perawatan`, `id_tt_siranap`, `id_siranap`, `deskripsi_siranap`, `covid`)

### 4c. Auto-populate dari `sk_bed`

Saat admin menekan **"Tampilkan Data"**, sistem melakukan dua query sekaligus:

1. Fetch data existing dari tabel `beds` (jika ada) → isi baris tabel
2. Query `sk_bed WHERE class_room_id = ? AND kamar = ?` → ambil `id_tt_siranap` dan `covid` sebagai **nilai default** untuk baris baru

Nilai auto-populate dapat diedit oleh admin sebelum disimpan.

### 4d. Mode Indikator & Fitur Tabel

- **Label mode** ditampilkan di atas tabel:
  - `🟡 Mode Edit — X baris ditemukan` (jika data existing ada)
  - `🟢 Mode Input Baru` (jika belum ada data)
- Tombol **"+ Tambah Baris"** — selalu tersedia di kedua mode
- Tombol **"× Hapus"** per baris — menghapus baris dari tampilan saja, tidak DELETE dari DB

> **Catatan:** Fitur hapus dari DB tidak disertakan di versi ini. Jika diperlukan, bisa ditambahkan di fase berikutnya.

---

## 5. Backend — Endpoint Baru

### 5a. Daftar Endpoint

| Method | Path | Fungsi |
|---|---|---|
| `GET` | `/api/beds/rooms` | Ambil DISTINCT `class_room_id` dari `sk_bed` (untuk dropdown 1) |
| `GET` | `/api/beds/kamar?class_room_id=X` | Ambil DISTINCT `kamar` berdasarkan `class_room_id` (untuk dropdown 2) |
| `GET` | `/api/beds/by-room?class_room_id=X&kamar=Y` | Fetch data existing dari `beds` + default dari `sk_bed` |
| `POST` | `/api/beds/upsert` | Bulk upsert ke tabel `beds` dalam satu transaksi |

### 5b. Struct Input — `BedsUpsertRequest`

```go
type BedsUpsertRequest struct {
    ClassRoomID string    `json:"class_room_id"`
    Kamar       string    `json:"kamar"`
    Rows        []BedRow  `json:"rows"`
}

type BedRow struct {
    BedID            int    `json:"bed_id"`
    RoomID           string `json:"room_id"`
    IDKelas          string `json:"id_kelas"`
    NmKelas          string `json:"nm_kelas"`
    IDPerawatan      string `json:"id_perawatan"`
    NmPerawatan      string `json:"nm_perawatan"`
    IDTTSiranap      string `json:"id_tt_siranap"`
    IDSiranap        string `json:"id_siranap"`
    DeskripsiSiranap string `json:"deskripsi_siranap"`
    Covid            string `json:"covid"`
    // Kolom berikut TIDAK ada di struct ini — di-inject backend:
    // org_unit_code → selalu "3404011"
    // class_room_id → dari BedsUpsertRequest.ClassRoomID
    // kamar         → dari BedsUpsertRequest.Kamar
}
```

### 5c. Alur `POST /api/beds/upsert`

```
1. Validasi payload:
   - class_room_id tidak boleh kosong
   - kamar boleh kosong (nullable — mengikuti data sk_bed)
   - rows tidak boleh kosong
   - bed_id, id_kelas, nm_kelas wajib ada di setiap baris

2. Fetch daftar bed_id existing untuk class_room_id + kamar dari DB
   → simpan sebagai map untuk lookup O(1)

3. Buka transaksi (*sql.Tx)

4. Loop setiap BedRow:
   a. Inject: org_unit_code = "3404011"
   b. Inject: class_room_id dari request
   c. Inject: kamar dari request
   d. Cek map existing:
      - Jika bed_id ADA di map  → UPDATE baris tersebut
      - Jika bed_id TIDAK ada   → INSERT baris baru

5. Jika semua baris berhasil → COMMIT
6. Jika ada satu baris gagal  → ROLLBACK seluruh batch

7. Return:
   { "saved": X, "inserted": Y, "updated": Z }
   atau
   { "error": "pesan error detail" }
```

### 5d. Alur `GET /api/beds/by-room`

```
1. Query tabel beds WHERE class_room_id = ? AND kamar = ?
   → return []BedRow (bisa kosong)

2. Query sk_bed WHERE class_room_id = ? AND kamar = ? AND tgl_berakhir IS NULL
   → ambil id_tt_siranap dan covid sebagai default values

3. Return:
   {
     "mode": "edit" | "new",
     "rows": [...data existing dari beds...],
     "defaults": {
       "id_tt_siranap": "...",
       "covid": "0"
     }
   }
```

---

## 6. Layer Repository

File baru: `internal/repository/beds_repository.go`

```go
// GetDistinctClassRooms mengambil daftar class_room_id unik dari sk_bed aktif.
func (r *BedsRepository) GetDistinctClassRooms() ([]string, error)

// GetKamarByClassRoom mengambil daftar kamar berdasarkan class_room_id dari sk_bed.
func (r *BedsRepository) GetKamarByClassRoom(classRoomID string) ([]string, error)

// GetBedsByRoom mengambil data beds existing + default dari sk_bed.
func (r *BedsRepository) GetBedsByRoom(classRoomID, kamar string) (BedsRoomResult, error)

// UpsertBeds melakukan INSERT/UPDATE beds dalam satu transaksi.
func (r *BedsRepository) UpsertBeds(req BedsUpsertRequest) (UpsertResult, error)
```

---

## 7. Struktur File Baru

```
internal/
└── repository/
    └── beds_repository.go      ← GetDistinctClassRooms, GetKamarByClassRoom,
                                   GetBedsByRoom, UpsertBeds

internal/
└── handler/
    └── beds_handler.go         ← handler untuk /api/beds/*

static/
└── index.html                  ← tambah Tab 6 (Alpine.js)
```

---

## 8. Task List Implementasi

### Fase A — Backend Repository

- [ ] Buat `internal/repository/beds_repository.go`
- [ ] Implementasi `GetDistinctClassRooms() ([]string, error)`
  - [ ] `SELECT DISTINCT class_room_id FROM sk_bed WHERE tgl_berakhir IS NULL ORDER BY class_room_id`
- [ ] Implementasi `GetKamarByClassRoom(classRoomID string) ([]string, error)`
  - [ ] `SELECT DISTINCT kamar FROM sk_bed WHERE class_room_id = ? AND tgl_berakhir IS NULL`
- [ ] Implementasi `GetBedsByRoom(classRoomID, kamar string) (BedsRoomResult, error)`
  - [ ] Query `beds WHERE class_room_id = ? AND kamar = ?`
  - [ ] Query `sk_bed` untuk default `id_tt_siranap` dan `covid`
  - [ ] Return mode (`edit`/`new`), rows, dan defaults
- [ ] Implementasi `UpsertBeds(req BedsUpsertRequest) (UpsertResult, error)`
  - [ ] Fetch existing `bed_id` untuk class_room_id + kamar
  - [ ] Buka `*sql.Tx`
  - [ ] Loop: inject `org_unit_code`, `class_room_id`, `kamar`; UPDATE jika ada, INSERT jika baru
  - [ ] COMMIT jika sukses, ROLLBACK jika gagal
  - [ ] Return jumlah inserted & updated

### Fase B — Backend Handler & Routing

- [ ] Buat `internal/handler/beds_handler.go`
- [ ] Implementasi handler `GET /api/beds/rooms`
- [ ] Implementasi handler `GET /api/beds/kamar`
  - [ ] Validasi query param `class_room_id` tidak kosong
- [ ] Implementasi handler `GET /api/beds/by-room`
  - [ ] Validasi query params `class_room_id` dan `kamar` tidak kosong
- [ ] Implementasi handler `POST /api/beds/upsert`
  - [ ] Validasi payload wajib (`class_room_id`, `rows`) — `kamar` boleh kosong
  - [ ] Validasi per baris: `bed_id`, `id_kelas`, `nm_kelas` tidak boleh kosong
  - [ ] Panggil `UpsertBeds`, return hasil atau error
- [ ] Daftarkan keempat route di `main.go` / router utama

### Fase C — Frontend Tab 6

- [ ] Tambah Tab 6 "Manajemen Beds" di navigasi dashboard
- [ ] Implementasi **Header Selector** (Alpine.js)
  - [ ] Dropdown `class_room_id` — fetch dari `GET /api/beds/rooms` saat tab dibuka
  - [ ] Dropdown `kamar` — fetch dari `GET /api/beds/kamar?class_room_id=X` saat class_room_id berubah
  - [ ] Tombol "Tampilkan Data" → fetch `GET /api/beds/by-room`
- [ ] Implementasi **Label Mode Indikator**
  - [ ] `🟡 Mode Edit — X baris ditemukan`
  - [ ] `🟢 Mode Input Baru`
- [ ] Implementasi **Tabel Spreadsheet** (Alpine.js)
  - [ ] 10 kolom input + 2 kolom read-only (`class_room_id`, `kamar`)
  - [ ] Urutan kolom sesuai struktur tabel `beds`
  - [ ] `bed_id` — input number
  - [ ] `id_tt_siranap` dan `covid` — auto-populate dari defaults, bisa diedit
  - [ ] `covid` — toggle (0/1)
  - [ ] Tombol "+ Tambah Baris" — append row kosong (dengan defaults ter-populate)
  - [ ] Tombol "× Hapus" per baris — hapus dari tampilan saja
- [ ] Implementasi **tombol Simpan**
  - [ ] POST ke `/api/beds/upsert`
  - [ ] Notifikasi sukses: `✅ X baris disimpan (Y inserted, Z updated)`
  - [ ] Notifikasi gagal: `❌ Gagal menyimpan — [pesan error]`

### Fase D — Testing

- [ ] Verifikasi dropdown `kamar` terfilter benar saat `class_room_id` berubah
- [ ] Verifikasi auto-populate `id_tt_siranap` dan `covid` dari `sk_bed`
- [ ] Test Mode Edit — data existing tampil di tabel, bisa diubah
- [ ] Test Mode Input Baru — tabel kosong, siap diisi
- [ ] Verifikasi `org_unit_code = "3404011"` ter-inject di semua baris
- [ ] Test UPDATE baris existing (bed_id sudah ada di DB)
- [ ] Test INSERT baris baru (bed_id belum ada di DB)
- [ ] Simulasi satu baris gagal → pastikan rollback terjadi
- [ ] Verifikasi `class_room_id` dan `kamar` read-only di tabel (tidak bisa diubah dari frontend)
- [ ] Test Tab 6 tampil dan berfungsi di `http://localhost:9271`

---

## 9. Ringkasan Progres Fitur Baru

| Fase | Deskripsi | Status |
|---|---|---|
| Fase A | Backend Repository (4 fungsi) | ⬜ Belum mulai |
| Fase B | Backend Handler & Routing (4 endpoint) | ⬜ Belum mulai |
| Fase C | Frontend Tab 6 (Selector, Tabel, Simpan) | ⬜ Belum mulai |
| Fase D | Testing & Verifikasi | ⬜ Belum mulai |

---

## 10. Relasi dengan Fitur Sebelumnya

| Fitur | Tab | Tabel DB | Ketergantungan |
|---|---|---|---|
| Worker Sync | Tab 4 | `sk_bed`, `beds`, `pasien_visitation` | Membaca `beds` untuk kalkulasi terisi |
| Manajemen SK | Tab 5 | `sk_bed` | **Sumber `class_room_id` dan `kamar`** untuk Tab 6 |
| Manajemen Beds | Tab 6 | `beds` | Bergantung pada data `sk_bed` yang sudah diinput via Tab 5 |

> **Urutan input yang disarankan:** Tab 5 (input SK TT) → Tab 6 (input Beds) → Worker Sync berjalan otomatis dengan data lengkap.