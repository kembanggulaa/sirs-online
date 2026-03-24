# Arsitektur & Task List
## Sistem Bridging RS Online V3 — RSUD Sleman

> Sistem sinkronisasi data tempat tidur otomatis antara **SIMRS (SQL Server)** dan **RS Online Kemenkes**.

---

## 1. Stack Teknologi

| Komponen      | Teknologi                              |
|---------------|----------------------------------------|
| Back-end      | Golang 1.2x (Goroutines & Channels)    |
| Configuration | Viper (manajemen `.env`)               |
| Database      | Microsoft SQL Server                   |
| Front-end     | Tailwind CSS + Alpine.js               |
| Automation    | Ticker-based Worker (interval 2 jam)   |
| Logging       | File `.log` (persistent ke disk)       |

---

## 2. Arsitektur Worker Pool

Aplikasi menginisialisasi sebuah **Dispatcher** yang mengatur antrean tugas melalui _job queue_.

```
┌─────────────┐     ┌──────────────┐     ┌────────────────────────────┐
│   Ticker    │────▶│  Dispatcher  │────▶│        Worker Pool         │
│ (setiap 2j) │     │  (Job Queue) │     │                            │
└─────────────┘     └──────────────┘     │  1. Buka 1 koneksi DB      │
         ▲                               │  2. Deteksi SK aktif (Q0)  │
         │                               │  3. Jalankan Query Temp    │
    Manual Trigger                       │  4. Jalankan Query Utama   │
    (Sync Now)                           │  5. Kalkulasi bed terisi   │
                                         │  6. Generate X-Timestamp   │
                                         │  7. PUT → API Kemenkes     │
                                         │     (retry maks 2x)        │
                                         │  8. Tulis ke file .log     │
                                         └────────────────────────────┘
```

### Komponen Utama

**Job**
- Dipicu oleh Ticker (otomatis 2 jam) atau Manual Trigger dari dashboard.
- Membawa konteks: konfigurasi koneksi, SK number aktif.

**Worker**
- Menerima `Job` dari channel.
- Mengeksekusi **Query Temp + Query Utama dalam satu transaksi** (satu koneksi DB).
- Kalkulasi `terisi` vs. `jumlah` per ruangan.
- Menghasilkan header `X-Timestamp` (Unix epoch, contoh: `"1774334805"`).
- Mengirim `PUT` request ke API Kemenkes (retry maks **2x** jika gagal).
- Mencatat hasil (sukses/gagal) ke file `.log`.

**Ticker**
- Pemicu otomatis setiap **2 jam** (dapat dikonfigurasi via `.env`).
- Jika job sebelumnya masih berjalan, tick berikutnya **di-skip** (tidak menumpuk).

**Retry Strategy & Response Handling**
- Setiap `PUT` request diretry maksimal **2x** dengan jeda singkat (5 detik).
- Jika HTTP 200 → catat di log: `[SUCCESS] Ruang X berhasil diupdate`.
- Jika setelah 2x retry masih gagal → catat di log: `[FAILED] Ruang X gagal diupdate`, lanjut tunggu jadwal 2 jam berikutnya.

---

## 3. Layer Database (Repository)

### ⚠️ Aturan Koneksi Penting

Karena menggunakan SQL Server **temp table** (`#temp_ranap`) yang hanya hidup dalam satu sesi, kedua query **HARUS** dijalankan pada **satu koneksi yang sama** menggunakan `*sql.Tx` (database transaction). Jangan gunakan connection pool terpisah untuk keduanya.

### Query 0 — Deteksi SK Aktif (Pre-query)

Dijalankan **sebelum** Query 1, setiap kali worker berjalan. Mengambil `sk_no` yang masih aktif berdasarkan kolom `tgl_berakhir IS NULL`.

```sql
SELECT DISTINCT sk_no FROM sk_bed WHERE tgl_berakhir IS NULL
```

> Hasil `sk_no` ini digunakan sebagai parameter `@sk_no` di Query 1 dan Query 2.  
> Jika hasilnya lebih dari satu, nilai pertama diambil dan **ditampilkan di Tab 4 Dashboard** sebagai konfirmasi admin.  
> **Jika tidak ada hasil** (tidak ada SK aktif) → worker berhenti, catat error di log: `[ERROR] Tidak ada SK aktif ditemukan di tabel sk_bed`.

### Query 1 — Membuat Temp Table Pasien Rawat Inap

```sql
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
    WHERE sk_no = @sk_no          -- dari Query 0 (auto-detect: tgl_berakhir IS NULL)
      AND tgl_berakhir IS NULL     -- validasi ganda: pastikan SK masih aktif
      AND class_room_id <> 'NI.BX'
  )
ORDER BY kamar
```

> Menghasilkan daftar tempat tidur yang **sedang terisi** oleh pasien rawat inap aktif.

### Query 2 — Query Utama Ketersediaan Bed

```sql
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
    ISNULL(t.terisi, 0) AS terisi       -- hasil JOIN dari #temp_ranap
FROM sk_bed sk
    INNER JOIN status_covid sc ON sc.id_tt = sk.id_tt_siranap
    LEFT JOIN (
        SELECT kamar, COUNT(*) AS terisi
        FROM #temp_ranap
        GROUP BY kamar
    ) t ON t.kamar = CONCAT(sk.class_room_id, sk.kamar)
WHERE sk.sk_no = @sk_no                 -- dari Query 0 (auto-detect: tgl_berakhir IS NULL)
  AND sk.tgl_berakhir IS NULL           -- validasi ganda: pastikan SK masih aktif
  AND sk.class_room_id <> 'NI.BX'
GROUP BY
    sk.id_tt_siranap, sk.class_room_id, sk.siranap, sk.jml_ruang_siranap,
    sk.kamar, sk.kelas_siranap, sk.ruang_siranap, sk.covid,
    sc.status, sc.konfirmasi, sc.antrian, t.terisi
ORDER BY sk.siranap, sk.ruang_siranap
```

> **Catatan optimasi:** Correlated subquery diganti dengan `LEFT JOIN + GROUP BY` agar tidak dieksekusi per-baris.

### Struct Output — `BedSiranap`

```go
type BedSiranap struct {
    IDTTSiranap        string  `json:"id_tt"`
    ClassRoomID        string  `json:"-"`               // internal, tidak dikirim ke API
    Siranap            string  `json:"ruang"`
    JmlRuang           int     `json:"jumlah_ruang"`
    Kelas              string  `json:"-"`               // internal, untuk display dashboard
    Kamar              string  `json:"-"`               // internal, untuk display dashboard
    KelasSiranap       string  `json:"-"`               // internal, untuk display dashboard
    Jumlah             int     `json:"jumlah"`
    Terisi             int     `json:"terpakai"`
    Status             int     `json:"terpakai_suspek"` // dari sc.status
    Konfirmasi         int     `json:"terpakai_konfirmasi"` // dari sc.konfirmasi
    Antrian            int     `json:"antrian"`
    Covid              int     `json:"covid"`           // 0 atau 1
    // Field opsional — dikirim dengan nilai "0" (belum wajib)
    Prepare            int     `json:"prepare"`
    PreparePlan        int     `json:"prepare_plan"`
}
```

Hasil query disimpan **in-memory** sebagai `[]BedSiranap` — tidak memerlukan database sementara eksternal (SQLite, dsb.).

---

## 4. Konfigurasi (`.env`)

```env
# Database SIMRS
DB_HOST=
DB_PORT=1433
DB_USER=
DB_PASS=
DB_NAME=

# API RS Online Kemenkes
API_URL=https://sirs.kemkes.go.id/fo/index.php/Fasyankes
API_RS_ID=
API_PASS=

# Operasional
APP_PORT=9271           # Port dashboard (non-standar, hanya akses lokal)
SYNC_INTERVAL_HOURS=2   # Interval ticker (jam)
RETRY_MAX=2             # Jumlah maksimum retry per request
LOG_FILE=logs/sirs.log  # Path file log
# SK_NO tidak dikonfigurasi di sini — diambil otomatis dari DB (tgl_berakhir IS NULL)
```

---

## 5. Deployment — Windows Service

Aplikasi di-deploy sebagai **Windows Service** menggunakan library `golang.org/x/sys/windows/svc`.

| Aspek | Detail |
|---|---|
| **Start** | Otomatis saat Windows boot, tanpa perlu login operator |
| **Dashboard** | Selalu aktif dan bisa diakses kapan saja via browser |
| **Sync Now** | Bisa digunakan kapan saja karena proses selalu hidup |
| **Recovery** | Dikonfigurasi restart otomatis bila crash |
| **Install** | `sc create sirs-online binPath= "C:\path\sirs-online.exe"` |
| **Start/Stop** | `sc start sirs-online` / `sc stop sirs-online` |

---

## 6. API RS Online Kemenkes

**Base URL:** `https://sirs.kemkes.go.id/fo/index.php/Fasyankes`

### Headers (wajib di semua method)

| Header        | Keterangan                              |
|---------------|-----------------------------------------|
| `X-rs-id`     | ID rumah sakit (dari `.env`)            |
| `X-pass`      | Password API (dari `.env`)              |
| `X-Timestamp` | Waktu saat ini format UTC ISO 8601      |

---

### GET — Referensi Tempat Tidur Kemenkes

```
GET https://sirs.kemkes.go.id/fo/index.php/Fasyankes/tempat_tidur
```

> Mengambil daftar referensi tempat tidur dari Kemenkes. Digunakan di **Tab 2 — Master Referensi**.

---

### GET — Data Tempat Tidur yang Sudah Diinputkan

```
GET https://sirs.kemkes.go.id/fo/index.php/Fasyankes
```

> URL ini **khusus** untuk mengambil data tempat tidur yang sudah pernah dikirimkan oleh RS ke Kemenkes.

---

### POST — Tambah Data Tempat Tidur

```
POST https://sirs.kemkes.go.id/fo/index.php/Fasyankes
```

**Body (JSON):**

```json
{
  "id_tt"               : "39",
  "ruang"               : "NICU Dengan Ventilator",
  "jumlah_ruang"        : "1",
  "jumlah"              : "2",
  "terpakai"            : "1",
  "terpakai_suspek"     : "0",
  "terpakai_konfirmasi" : "0",
  "antrian"             : "0",
  "prepare"             : "0",
  "prepare_plan"        : "0",
  "covid"               : "0"
}
```

---

### PUT — Update Data Tempat Tidur *(Digunakan oleh Worker)*

```
PUT https://sirs.kemkes.go.id/fo/index.php/Fasyankes/{id_tt}
```

> `{id_tt}` diisi dengan nilai `id_tt_siranap` dari `sk_bed` (contoh: `.../Fasyankes/39`).

**Body (JSON):**

```json
{
  "id_tt"               : "39",        // id_tt_siranap dari sk_bed
  "ruang"               : "Nusa Indah 3",  // hasil IIF(kamar IS NULL, ruang, ruang+kamar)
  "jumlah_ruang"        : "1",        // jml_ruang_siranap
  "jumlah"              : "2",        // SUM(sk.bed)
  "terpakai"            : "1",        // COUNT dari #temp_ranap
  "terpakai_suspek"     : "0",        // sc.status
  "terpakai_konfirmasi" : "0",        // sc.konfirmasi
  "antrian"             : "0",        // sc.antrian
  "prepare"             : "0",        // belum wajib, kirim "0"
  "prepare_plan"        : "0",        // belum wajib, kirim "0"
  "covid"               : "0"         // sk.covid
}
```

> Satu request `PUT` dikirim **per baris** dari `[]BedSiranap`. Jika ada 20 ruangan → 20 request PUT per siklus sync.

---

## 7. Tampilan Dashboard

Dashboard terdiri dari **4 Tab** utama:

### Tab 1 — Info Ruang
> Data real-time dari hasil `[]BedSiranap` (in-memory) — menampilkan tabel ruangan, jumlah bed, terisi, dan status COVID.

### Tab 2 — Master Referensi
> Data referensi tempat tidur dari Kemenkes via `GET /tempat_tidur` — **read-only**.

### Tab 3 — Manajemen TT
> Form untuk operasi `POST` (tambah TT baru) dan `PUT` (update manual TT) ke API Kemenkes. Dipisah dari Tab 2 agar tidak tercampur.

### Tab 4 — Operasional & Worker
> Panel pemantauan dan kontrol worker.

- 📋 Log aktivitas (baca dari file `.log`)
- ▶️ Tombol manual **"Sync Now"** (Manual Trigger)
- 🟢 Indikator status Worker (`Running` / `Idle`)
- ℹ️ Info jadwal sync berikutnya
- 🔖 **Tampilan SK aktif** — menampilkan `sk_no` yang terdeteksi dari DB (`tgl_berakhir IS NULL`) sebagai konfirmasi admin sebelum/setelah sync

---

## 7. Rencana Tugas (Task List)

### Fase 1 — Setup & Konfigurasi

- [ ] Inisialisasi proyek Go dan install library (`viper`, `go-mssqldb`, `resty/v2`)
- [ ] Buat `config/config.go` untuk mapping `.env`
- [ ] Buat struktur direktori proyek

### Fase 2 — Database & Query (Repository)

- [ ] Buat `internal/repository/bed_repository.go`
- [ ] Implementasi `GetActiveSKNo() (string, error)` — query `sk_bed WHERE tgl_berakhir IS NULL`
- [ ] Implementasi `GetBedAvailability(skNo string) ([]BedSiranap, error)`:
    - [ ] Buka transaksi (`*sql.Tx`) pada satu koneksi
    - [ ] Jalankan **Query 0** — deteksi SK aktif
    - [ ] Jalankan **Query 1** — buat `#temp_ranap`
    - [ ] Jalankan **Query 2** — ambil data ketersediaan bed (dengan LEFT JOIN)
    - [ ] Return `[]BedSiranap`

### Fase 3 — Worker Pool & Automation

- [ ] Buat `internal/worker/dispatcher.go` dan `worker.go`
- [ ] Implementasi `time.NewTicker` untuk interval (dari config)
- [ ] Logic skip jika job sebelumnya masih berjalan
- [ ] Retry logic (maks 2x, jeda 5 detik)
- [ ] Kirim `PUT` ke API Kemenkes dengan headers yang benar
- [ ] Tulis hasil ke file `.log`

### Fase 4 — REST API Internal

- [ ] Endpoint `GET /api/beds` — return data `[]BedSiranap` (untuk Tab 1)
- [ ] Endpoint `GET /api/logs` — return isi file `.log` (untuk Tab 3)
- [ ] Endpoint `POST /api/sync` — trigger manual sync (untuk Tab 3)
- [ ] Endpoint `GET /api/worker/status` — return status `Running`/`Idle`
- [ ] Endpoint `GET /api/sk-active` — return `sk_no` aktif saat ini (untuk Tab 3)

### Fase 5 — Front-end Dashboard

- [ ] Desain header & layout (Tailwind CSS)
- [ ] Implementasi Tab System Alpine.js (4 tab)
- [ ] Tab 1: tabel Info Ruang (fetch dari `/api/beds`)
- [ ] Tab 2: Master Referensi read-only (fetch GET dari `/api/proxy/referensi`)
- [ ] Tab 3: Form POST & PUT ke API Kemenkes
- [ ] Tab 4: Log, tombol Sync Now, indikator status, SK aktif

### Fase 6 — Uji Coba & Testing

- [ ] Verifikasi dua query berjalan dalam satu transaksi
- [ ] Simulasi no active SK → validasi error log & worker berhenti
- [ ] Simulasi kegagalan jaringan → validasi retry & log `[FAILED]`
- [ ] Verifikasi format `X-Timestamp` sebagai Unix epoch (seconds)
- [ ] Verifikasi response PUT 200 tercatat sebagai `[SUCCESS]` di log
- [ ] Test Manual Trigger dari dashboard
- [ ] Akses dashboard di `http://localhost:9271`

---

## Ringkasan Progres

| Fase   | Deskripsi               | Status          |
|--------|-------------------------|-----------------|
| Fase 1 | Setup & Konfigurasi     | ⬜ Belum mulai  |
| Fase 2 | Database & Query        | ⬜ Belum mulai  |
| Fase 3 | Worker Pool             | ⬜ Belum mulai  |
| Fase 4 | REST API Internal       | ⬜ Belum mulai  |
| Fase 5 | Front-end Dashboard     | ⬜ Belum mulai  |
| Fase 6 | Testing                 | ⬜ Belum mulai  |