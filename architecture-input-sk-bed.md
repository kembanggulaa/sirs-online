# Arsitektur & Task List — Fitur Input SK Tempat Tidur
## Sistem Bridging RS Online V3 — RSUD Sleman

> Fitur tambahan untuk menginputkan data SK Tempat Tidur terbaru langsung ke tabel `sk_bed` melalui dashboard, tanpa perlu akses langsung ke database.

---

## 1. Ringkasan Fitur

Fitur **Manajemen SK TT** memungkinkan admin IT untuk memasukkan data SK Tempat Tidur baru ke tabel `sk_bed` melalui antarmuka spreadsheet di browser. Alur terdiri dari dua langkah utama:

1. **Step 1 — Header SK** → admin mengisi `sk_no` dan `tgl_berlaku`. Opsi **"Import Excel"** tersedia untuk mempopulasi baris secara massal.
2. **Step 2 — Input Baris** → admin mengisi tabel spreadsheet 16 kolom (atau hasil import Excel), dengan fitur duplikasi baris untuk efisiensi.
3. **Step 3 — Preview & Konfirmasi** → ringkasan data dan deteksi SK lama yang akan "pensiun" sebelum disimpan ke DB.
4. **Bulk Insert (Atomic)** → semua baris diinsert dengan perhitungan `MAX(id)` otomatis, dan SK lama di-deaktivasi dalam satu transaksi SQL.

---

## 2. Posisi di Dashboard

Fitur ini ditempatkan sebagai **Tab 5 — Manajemen SK**, tab baru setelah Tab 4 (Operasional & Worker) yang sudah ada.

```
[ Tab 1: Info Ruang ] [ Tab 2: Master Referensi ] [ Tab 3: Manajemen TT ] [ Tab 4: Operasional ] [ Tab 5: Manajemen SK ✨ ]
```

---

## 3. Alur Kerja (Flow)

```
┌─────────────────────────────────────────────────────────────────────┐
│                        TAB 5 — Manajemen SK                        │
│                                                                     │
│  STEP 1 — Header SK                                                 │
│  ┌─────────────────────────────────┐                                │
│  │ sk_no      : [________________] │  ← diisi manual admin          │
│  │ tgl_berlaku: [  date picker   ] │  ← tanggal SK berlaku          │
│  │                                 │                                │
│  │ [ 📂 Unggah Excel ]             │  ← Optional: populate Step 2   │
│  │        [ Lanjut ke Input → ]    │                                │
│  └─────────────────────────────────┘                                │
│              │                                                      │
│              ▼                                                      │
│  STEP 2 — Input Baris sk_bed                                        │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ SK No: SK/001/2026  |  tgl_berlaku: 2026-01-01  [Edit ←]  │    │
│  ├─────────────────────────────────────────────────────────────┤    │
│  │ Tabel Spreadsheet (16 kolom, scrollable horizontal)         │    │
│  │ + fitur duplikasi baris (copy row)                          │    │
│  │ + tombol hapus per baris                                    │    │
│  ├─────────────────────────────────────────────────────────────┤    │
│  │ [ + Tambah Baris ]  [ ⧉ Duplikasi Baris Terakhir ]          │    │
│  │                                     [ Preview → ]           │    │
│  └─────────────────────────────────────────────────────────────┘    │
│              │                                                      │
│              ▼ klik "Preview"                                       │
│  STEP 3 — Preview & Konfirmasi                                      │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ SK No   : SK/001/2026                                       │    │
│  │ Berlaku : 2026-01-01                                        │    │
│  │ Total   : 52 baris akan diinsert                            │    │
│  │ Pensiun : SK/999/2025 (tgl_berakhir: 2025-12-31)            │    │
│  │                                                             │    │
│  │ [Tabel preview read-only]                                   │    │
│  │                                                             │    │
│  │ [ ← Kembali & Edit ]        [ ✓ Konfirmasi & Simpan ]      │    │
│  └─────────────────────────────────────────────────────────────┘    │
│              │                                                      │
│              ▼ POST /api/sk/import                                  │
│  ┌─────────────────────────────────────────────────────────────┐    │
│  │ ✅ Berhasil: 52 baris diinsert ke tabel sk_bed              │    │
│  │    atau                                                     │    │
│  │ ❌ Gagal: [pesan error] — tidak ada data yang tersimpan     │    │
│  └─────────────────────────────────────────────────────────────┘    │
└─────────────────────────────────────────────────────────────────────┘
```

---

## 4. Struktur Kolom Tabel Input

### 4a. Header Form (Step 1) — diisi sekali, berlaku semua baris

| Field | Input | Keterangan |
|---|---|---|
| `sk_no` | text | Nomor SK, diketik manual (contoh: `SK/001/2026`) |
| `tgl_berlaku` | date picker | Tanggal SK mulai berlaku |

### 4b. Tabel Spreadsheet (Step 2) — diisi per baris

Urutan kolom mengikuti urutan tabel `sk_bed` (minus kolom auto):

| # | Kolom DB | Input Type | Keterangan |
|---|---|---|---|
| 1 | `tgl_berlaku` | — | **Auto** — diambil dari Step 1 |
| 2 | `clinic_id` | text | Kode klinik/instalasi |
| 3 | `class_room_id` | text | Kode bangsal |
| 4 | `kelas` | text | Kelas perawatan internal |
| 5 | `bed` | number | Jumlah bed |
| 6 | `sk_no` | — | **Auto** — diambil dari Step 1 |
| 7 | `tgl_berakhir` | — | **Auto NULL** — tidak ditampilkan |
| 8 | `id_tt_siranap` | text | ID referensi Kemenkes |
| 9 | `ruang_siranap` | text | Nama ruang di sistem Kemenkes |
| 10 | `kelas_siranap` | text | Kelas di sistem Kemenkes |
| 11 | `covid` | toggle (0/1) | Flag ruang COVID |
| 12 | `siranap` | text | Nama siranap |
| 13 | `jml_ruang_siranap` | number | Jumlah ruang siranap |
| 14 | `kodekelas` | text | Kode kelas |
| 15 | `namakelas` | text | Nama kelas |
| 16 | `namaruang` | text | Nama ruang lengkap |
| 17 | `kris` | text | Kolom KRIS |
| 18 | `kamar` | text | Nomor/kode kamar — **kolom paling kanan** |
| 19 | `id` | — | **Auto-Sequence** — dihitung Backend |

**Kolom auto (tidak tampil di tabel):** `id` (dikalkulasi backend), `sk_no` (dari Step 1), `tgl_berlaku` (dari Step 1), `tgl_berakhir` (selalu `NULL`).

**Total kolom di database: 19 kolom (15 kolom diisi manual per baris)**

### 4c. Fitur Duplikasi Baris

Karena pola umum adalah satu bangsal dengan banyak kamar (contoh: Alamanda 1 kelas 3 dengan 6 kamar → 6 baris identik kecuali kolom `kamar`), tabel menyediakan:

- **Tombol "⧉ Duplikasi"** per baris → menyalin baris tersebut ke bawahnya
- Kolom `kamar` di posisi paling kanan agar mudah ditemukan dan diedit setelah duplikasi
- **Tombol "× Hapus"** per baris untuk menghapus baris yang tidak diperlukan

---

## 5. Backend — Endpoint Baru

### 5a. Daftar Endpoint

| Method | Path | Fungsi |
|---|---|---|
| `POST` | `/api/sk/preview` | Validasi payload, return ringkasan (belum insert ke DB) |
| `POST` | `/api/sk/import` | Bulk insert ke tabel `sk_bed` dalam satu transaksi |
| `GET` | `/api/sk/list` | Ambil daftar `sk_no` yang sudah ada di DB (untuk referensi admin) |

### 5b. Struct Input — `SKImportRequest`

```go
type SKImportRequest struct {
    SKNo      string      `json:"sk_no"`
    Rows      []SKBedRow  `json:"rows"`
}

type SKBedRow struct {
    TglBerlaku      string `json:"tgl_berlaku"`
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
    // sk_no & tgl_berakhir TIDAK ada di struct ini:
    // sk_no      → diambil dari SKImportRequest.SKNo
    // tgl_berakhir → selalu NULL, di-set oleh backend
}
```

### 5c. Alur `POST /api/sk/import` (Atomic Transaction)

```
1. Validasi payload — sk_no tidak boleh kosong, rows tidak boleh kosong
2. Buka transaksi (*sql.Tx)
3. Ambil ID terakhir: SELECT MAX(id) FROM sk_bed
4. Deteksi SK aktif saat ini (sk_no_lama) yang tgl_berakhir IS NULL
5. Update SK lama: UPDATE sk_bed SET tgl_berakhir = (tgl_berlaku_baru - 1) WHERE sk_no = sk_no_lama
6. Loop setiap baris SKBedRow:
   a. id = max_id++
   b. Inject sk_no & tgl_berlaku dari request header
   c. Set tgl_berakhir = NULL
   d. Eksekusi INSERT INTO sk_bed (...)
7. Jika semua langkah berhasil → COMMIT
8. Jika ada satu langkah gagal → ROLLBACK seluruh batch
9. Return: { "inserted": 52, "sk_no": "SK/001/2026" }
```

### 5d. Logika Manajemen ID
Karena kolom `id` di `sk_bed` tidak auto-increment, backend akan selalu menjamin urutan ID dengan mengambil nilai maksimal saat ini di dalam transaksi yang sama. Data dari Excel yang memiliki `id` akan diabaikan/di-overwrite oleh backend untuk keamanan sinkronisasi.

### 5e. Alur `POST /api/sk/preview`

```
1. Terima payload SKImportRequest
2. Tidak melakukan INSERT
3. Return ringkasan:
   {
     "sk_no": "SK/001/2026",
     "tgl_berlaku": "2026-01-01",
     "total_rows": 52,
     "rows": [ ...data lengkap untuk ditampilkan di tabel preview... ]
   }
```

---

## 6. Layer Repository

File baru: `internal/repository/sk_repository.go`

```go
// BulkInsertSKBed menyimpan semua baris dalam satu transaksi.
// Jika satu baris gagal, seluruh batch di-rollback.
func (r *SKRepository) BulkInsertSKBed(req SKImportRequest) (int, error)

// GetSKList mengambil daftar sk_no unik dari tabel sk_bed (untuk referensi admin).
func (r *SKRepository) GetSKList() ([]string, error)
```

**Catatan:** Menggunakan `*sql.Tx` untuk memastikan atomicity — semua baris masuk atau tidak sama sekali.

---

## 7. Struktur File Baru

```
internal/
└── repository/
    └── sk_repository.go       ← BulkInsertSKBed, GetSKList

internal/
└── handler/
    └── sk_handler.go          ← handler untuk /api/sk/*

static/
└── index.html                 ← tambah Tab 5 (Alpine.js)
```

---

## 8. Task List Implementasi

### Fase A — Backend Repository

- [ ] Buat `internal/repository/sk_repository.go`
- [ ] Implementasi `BulkInsertSKBed(req SKImportRequest) (int, error)`
  - [ ] Buka `*sql.Tx`
  - [ ] Loop INSERT per baris dengan inject `sk_no` dan `tgl_berakhir = NULL`
  - [ ] COMMIT jika semua sukses, ROLLBACK jika ada yang gagal
- [ ] Implementasi `GetSKList() ([]string, error)`
  - [ ] Query `SELECT DISTINCT sk_no FROM sk_bed ORDER BY sk_no DESC`

### Fase B — Backend Handler & Routing

- [ ] Buat `internal/handler/sk_handler.go`
- [ ] Implementasi handler `POST /api/sk/preview`
  - [ ] Validasi payload (sk_no tidak kosong, rows tidak kosong)
  - [ ] Return ringkasan tanpa insert ke DB
- [ ] Implementasi handler `POST /api/sk/import`
  - [ ] Panggil `BulkInsertSKBed`
  - [ ] Return jumlah baris berhasil atau pesan error
- [ ] Implementasi handler `GET /api/sk/list`
  - [ ] Panggil `GetSKList`, return JSON array
- [ ] Daftarkan ketiga route di `main.go` / router utama

### Fase C — Frontend Tab 5

- [ ] Tambah Tab 5 "Manajemen SK" di navigasi dashboard
- [ ] Implementasi **Step 1 — Header Form** (Alpine.js)
  - [ ] Input `sk_no` (text)
  - [ ] Input `tgl_berlaku` (date picker)
  - [ ] Tombol "Lanjut ke Input →" → transisi ke Step 2
- [ ] Implementasi **Step 2 — Spreadsheet Table** (Alpine.js)
  - [ ] Label read-only `sk_no` dan `tgl_berlaku` di atas tabel
  - [ ] Tombol "← Edit" untuk kembali ke Step 1
  - [ ] Tabel 16 kolom, scrollable horizontal (Tailwind)
  - [ ] Tombol "+ Tambah Baris" — append row kosong
  - [ ] Tombol "⧉ Duplikasi" per baris — copy row di bawahnya
  - [ ] Tombol "× Hapus" per baris
  - [ ] Kolom `kamar` di posisi paling kanan
  - [ ] Toggle (checkbox) untuk kolom `covid` (0/1)
  - [ ] Tombol "Preview →" — POST ke `/api/sk/preview`
- [ ] Implementasi **Step 3 — Preview Panel**
  - [ ] Tampilkan ringkasan: sk_no, tgl_berlaku, total baris
  - [ ] Tabel preview read-only (seluruh data)
  - [ ] Tombol "← Kembali & Edit" → kembali ke Step 2
  - [ ] Tombol "✓ Konfirmasi & Simpan" → POST ke `/api/sk/import`
- [ ] Implementasi **notifikasi hasil**
  - [ ] Sukses: tampilkan jumlah baris berhasil diinsert
  - [ ] Gagal: tampilkan pesan error, data tidak tersimpan

### Fase D — Testing & Verifikasi

- [ ] Verifikasi bulk insert dalam satu transaksi (semua masuk atau rollback)
- [ ] Simulasi de-aktivasi SK lama (cek `tgl_berakhir` SK sebelumnya)
- [ ] Simulasi perhitungan `MAX(id)` untuk baris baru
- [ ] Test import Excel (format kolom 1-18)
- [ ] Test duplikasi baris → pastikan hanya kolom `kamar` yang perlu diubah
- [ ] Verifikasi `sk_no` dan `tgl_berakhir = NULL` ter-inject benar di semua baris
- [ ] Test input 50+ baris → performa bulk insert
- [ ] Verifikasi Tab 5 tampil dan berfungsi di `http://localhost:9271`

---

## 9. Ringkasan Progres Fitur Baru

| Fase | Deskripsi | Status |
|---|---|---|
| Fase A | Backend Repository (BulkInsert, GetSKList) | ⬜ Belum mulai |
| Fase B | Backend Handler & Routing (3 endpoint) | ⬜ Belum mulai |
| Fase C | Frontend Tab 5 (Step 1, 2, 3 + notifikasi) | ⬜ Belum mulai |
| Fase D | Testing & Verifikasi | ⬜ Belum mulai |