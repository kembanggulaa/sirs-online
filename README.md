# SIRS Online Bridging System V3

> Sistem otomatis untuk mensinkronisasi data ketersediaan tempat tidur dari **SIMRS (SQL Server)** ke **API RS Online Kemenkes**.

---

## 📋 Daftar Isi

1. [Apa Itu SIRS Online?](#1-apa-itu-sirs-online)
2. [Fitur Utama](#2-fitur-utama)
3. [Arsitektur Sistem](#3-arsitektur-sistem)
4. [Prasyarat](#4-prasyarat)
5. [Instalasi & Konfigurasi](#5-instalasi--konfigurasi)
6. [Menjalankan Aplikasi](#6-menjalankan-aplikasi)
7. [Dashboard — 6 Tab](#7-dashboard--6-tab)
8. [Mengelola SK & Import Excel (Tab 5)](#8-mengelola-sk--import-excel-tab-5)
9. [Endpoint API](#9-endpoint-api)
10. [Testing](#10-testing)
11. [Konvensi Git](#11-konvensi-git)
12. [Dokumentasi Lengkap](#12-dokumentasi-lengkap)

---

## 1. Apa Itu SIRS Online?

SIRS Online Bridging V3 adalah **layanan latar belakang** (*background service*) yang dibangun dengan Go untuk **RSUD Sleman**. Sistem ini berfungsi sebagai **jembatan** antara database SIMRS internal dan API publik Kemenkes.

### Cara Kerja Singkat

Setiap **2 jam** (atau manual), sistem:
1. Mendeteksi SK (Surat Keterangan) yang masih aktif dari database SIMRS
2. Mengambil data ketersediaan tempat tidur dari database
3. Mengirim laporan ke **API Kemenkes** (`sirs.kemkes.go.id`)

### Siapa yang Perlu Menggunakan?

- **Operator SIMRS** — mengelola SK dan memetakan tempat tidur
- **Administrator IT** — deploy dan monitoring server
- **Tim Yankes** — memantau data RS Online Kemenkes

---

## 2. Fitur Utama

| Fitur | Penjelasan |
|---|---|
| **Sinkronisasi Otomatis** | Worker pool berjalan setiap N jam (default: 2 jam) |
| **Sinkronisasi Manual** | Tombol "Sync Now" untuk trigger kapan saja |
| **Dashboard Web** | Antarmuka browser untuk monitoring dan kontrol (6 tab) |
| **Manajemen SK** | Buat, edit, dan import data Surat Keterangan via Excel |
| **Pemetaan Tempat Tidur** | Pemetaan `class_room_id` ke data Kemenkes |
| **Graceful Shutdown** | Berhenti dengan baik saat service dihentikan |
| **Windows Service** | Jalan 24/7 tanpa perlu login operator |
| **Structured Logging** | Log tersimpan di file, 200 baris terakhir bisa dilihat via API |

---

## 3. Arsitektur Sistem

```
sirs-online/
├── main.go                     # Titik masuk (console / Windows Service)
├── config/config.go            # Konfigurasi dari .env
├── internal/
│   ├── handler/                # Endpoint HTTP
│   │   ├── api_handler.go      # beds, logs, sync, worker status
│   │   ├── sk_handler.go      # Manajemen SK (list, detail, import)
│   │   ├── beds_handler.go     # Pemetaan tempat tidur
│   │   └── proxy_handler.go    # Proxy ke API Kemenkes
│   ├── worker/                 # Inti sistem sinkronisasi
│   │   ├── dispatcher.go       # Penjadwal ticker & antrian job
│   │   ├── worker.go           # Logika sync (DB → Kemenkes PUT)
│   │   └── client.go           # HTTP client ke Kemenkes
│   ├── repository/             # Query ke database SIMRS
│   │   ├── bed_repository.go  # GetActiveSKNo, GetBedAvailability
│   │   ├── sk_repository.go   # SK list, detail, bulk insert
│   │   └── beds_repository.go # Rooms, kamar, upsert mapping
│   └── logger/                 # Logger ke file
└── web/static/                 # Dashboard Alpine.js + Tailwind CSS
```

### Alur Sinkronisasi

```
[Ticker setiap 2 jam] ──atau── [Tombol Sync Now]
        │
        ▼
  Dispatcher.dispatch()
        │
        ▼
  Worker.processJob()
   ├── 1. GetActiveSKNo()        → Ambil SK aktif dari sk_bed
   ├── 2. GetBedAvailability()   → Ambil data bed dari SIMRS
   ├── 3. GET /Fasyankes         → Ambil mapping id_t_tt dari Kemenkes
   └── 4. PUT /Fasyankes/{id_tt} → Kirim data per ruangan (dengan retry)
```

---

## 4. Prasyarat

- **Go 1.23+** — [download di golang.org/dl](https://golang.org/dl/)
- **SQL Server SIMRS** — akses host, port, user, password, nama database
- **Kredensial API Kemenkes** — `API_RS_ID` dan `API_PASS` dari Kemenkes
- **Windows** — untuk mode Windows Service (opsional)
- ** direktori `logs/`** — dibuat manual sebelum pertama kali jalan

---

## 5. Instalasi & Konfigurasi

### 5.1 Clone & Install Dependencies

```bash
git clone <repository-url>
cd sirs-online
go mod tidy
```

### 5.2 Buat Folder Logs

```bash
mkdir logs
```

File log akan ditulis ke `logs/sirs.log`. Jika folder tidak ada, aplikasi bisa gagal atau log tidak tersimpan.

### 5.3 Buat File `.env`

```bash
copy .env.example .env
```

Edit file `.env` dengan nilai sebenarnya:

```env
# ── Database SIMRS (SQL Server) ──────────────────────────────────────
DB_HOST=localhost
DB_PORT=1433
DB_USER=sa
DB_PASS=P@ssw0rdDB
DB_NAME=db_simrs_utama

# ── API RS Online Kemenkes ────────────────────────────────────────────
API_URL=https://sirs.kemkes.go.id/fo/index.php
API_RS_ID=KODE_RS_ANDA
API_PASS=PASSWORD_API_RAHASIA

# ── Operasional ───────────────────────────────────────────────────────
APP_PORT=9271
SYNC_INTERVAL_HOURS=2
RETRY_MAX=2
LOG_FILE=logs/sirs.log
ORG_UNIT_CODE=KODE_RS_ANDA

# ── TLS (untuk API pemerintah dengan sertifikat self-signed) ───────────
TLS_SKIP_VERIFY=false

# ── Keamanan ──────────────────────────────────────────────────────────
DASHBOARD_ORIGIN=http://localhost:9271
MAX_BODY_BYTES=1048576
```

### Referensi Variabel `.env`

| Variabel | Default | Penjelasan |
|---|---|---|
| `DB_HOST` | — | Host SQL Server |
| `DB_PORT` | `1433` | Port SQL Server |
| `DB_USER` | — | Username database SIMRS |
| `DB_PASS` | — | Password database |
| `DB_NAME` | — | Nama database SIMRS |
| `API_URL` | — | Base URL API Kemenkes |
| `API_RS_ID` | — | Kode RS (header X-rs-id) |
| `API_PASS` | — | Password API Kemenkes |
| `APP_PORT` | `9271` | Port dashboard HTTP |
| `SYNC_INTERVAL_HOURS` | `2` | Interval sync otomatis (jam) |
| `RETRY_MAX` | `2` | Jumlah retry jika PUT gagal |
| `LOG_FILE` | `logs/sirs.log` | Lokasi file log |
| `TLS_SKIP_VERIFY` | `false` | Skip verifikasi TLS |
| `ORG_UNIT_CODE` | — | Kode RS untuk org_unit_code |
| `DASHBOARD_ORIGIN` | — | Origin CORS dashboard |
| `MAX_BODY_BYTES` | `1048576` | Maks ukuran body POST/PUT |

---

## 6. Menjalankan Aplikasi

### Mode Development (Console)

```bash
go run main.go
```

Buka dashboard di: **`http://localhost:9271`**

### Build Binary

```bash
go build -o sirs-online.exe .
./sirs-online.exe
```

### Mode Windows Service (Produksi)

```bash
# Daftar sebagai Windows Service (jalankan sebagai Administrator)
sc create SIRSOnline binPath="C:\path\to\sirs-online.exe" start=auto

# Jalankan service
sc start SIRSOnline

# Hentikan service
sc stop SIRSOnline

# Hapus service
sc delete SIRSOnline
```

> **Catatan:** Aplikasi mendeteksi mode secara otomatis. Jika login sebagai Administrator atau di Terminal interactif → jalan sebagai console app. Jika tidak → jalan sebagai Windows Service.

---

## 7. Dashboard — 6 Tab

Dashboard tersedia di `http://localhost:9271` dengan **6 tab**:

| Tab | Nama | Fungsi |
|---|---|---|
| **1** | Info Ruang | Data ketersediaan bed real-time dari hasil sync terakhir |
| **2** | Master Referensi | Data referensi tempat tidur dari Kemenkes (hanya baca) |
| **3** | Manajemen TT | Form untuk tambah/edit tempat tidur ke Kemenkes |
| **4** | Operasional & Worker | Log aktivitas, tombol Sync Now, status worker |
| **5** | SK Manajemen | Kelola Surat Keterangan (SK) — termasuk import Excel |
| **6** | Beds Management | Pemetaan `class_room_id` → kamar → `id_t_tt` |

### Tab Eksekutif

Selain dashboard utama, tersedia dashboard eksekutif di **`web/static/eksekutif.html`** untuk pemantauan level manajemen.

---

## 8. Mengelola SK & Import Excel (Tab 5)

Tab 5 digunakan untuk **membuat dan mengimport data Surat Keterangan (SK)** ke tabel `sk_bed` di database SIMRS.

### Langkah-Langkah Import via Excel

1. **Buka Tab 5 — SK Manajemen**
2. **Step 1**: Isi *Nomor SK Baru* dan *Tanggal Berlaku*
3. **Unggah File Excel**: Klik tombol **"Pilih File Excel"** atau drag-and-drop file `.xlsx`
4. **Step 2**: Data dari Excel akan muncul. Review dan perbaiki jika perlu.
5. **Step 3**: Klik **Simpan** — SK lama (jika ada) akan otomatis dipensiunkan.

### Format Kolom Excel (15 Kolom)

File Excel harus memiliki **header row** dengan kolom-kolom berikut:

| No | Nama Kolom | Tipe | Keterangan |
|---|---|---|---|
| 1 | `clinic_id` | text | ID klinik (boleh kosong) |
| 2 | `class_room_id` | text | ID bangsal/ruangan |
| 3 | `kelas` | text | Kelas perawatan |
| 4 | `bed` | number | Jumlah tempat tidur |
| 5 | `id_tt_siranap` | text | ID TT dari Kemenkes |
| 6 | `ruang_siranap` | text | Nama ruangan Siranap |
| 7 | `kelas_siranap` | text | Kelas Siranap |
| 8 | `covid` | number | Flag COVID (0 atau 1) |
| 9 | `siranap` | text | Nama Siranap |
| 10 | `jml_ruang_siranap` | number | Jumlah ruang (default 1) |
| 11 | `kodekelas` | text | Kode kelas |
| 12 | `namakelas` | text | Nama kelas |
| 13 | `namaruang` | text | Nama ruang (fallback jika kamar kosong) |
| 14 | `kris` | text | Indikator kris |
| 15 | `kamar` | text | Nomor kamar |

### Template Excel

Template siap pakai tersedia di: **`example/sk_bed_import_template.xlsx`**

Buka file tersebut di Excel atau Google Sheets, edit datanya sesuai kebutuhan RS Anda, lalu save.

### Catatan Penting

- **Urutan kolom fleksibel** — sistem mencari kolom berdasarkan nama header (case-insensitive)
- **Baris kosong** akan dilewati secara otomatis
- Jika **SK lama masih aktif**, sistem akan otomatis memensiunkannya (tgl_berakhir = H-1 dari tgl_berlaku SK baru)
- Proses ini **idempotent** — import ulang dengan SK yang sama akan menimpa data lama

---

## 9. Endpoint API

Semua endpoint berjalan di `http://localhost:{APP_PORT}`.

### Dashboard

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/` | Halaman dashboard utama |

### Sinkronisasi & Monitoring

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/beds` | Data ketersediaan bed |
| `GET` | `/api/logs` | 200 baris terakhir dari file log |
| `POST` | `/api/sync` | Trigger sinkronisasi manual |
| `GET` | `/api/worker/status` | Status worker (`Running` / `Idle`) |
| `GET` | `/api/sk-active` | SK aktif yang sedang digunakan |
| `GET` | `/api/healthz` | Health check |

### Manajemen SK (Tab 5)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/sk/list` | Daftar semua nomor SK |
| `GET` | `/api/sk/detail?sk_no=<SK>` | Detail SK tertentu |
| `POST` | `/api/sk/preview` | Preview data sebelum import |
| `POST` | `/api/sk/import` | Import data SK ke database |

### Manajemen Tempat Tidur (Tab 6)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/beds/rooms` | Daftar `class_room_id` |
| `GET` | `/api/beds/kamar?class_room_id=<ID>` | Daftar kamar |
| `GET` | `/api/beds/by-room?class_room_id=<ID>` | Data beds per ruangan |
| `POST` | `/api/beds/upsert` | Simpan/update mapping beds |

### Proxy Kemenkes

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/proxy/referensi` | Referensi TT dari Kemenkes |
| `GET` | `/api/proxy/fasyankes` | Data Fasyankes yang sudah diinput |
| `POST` | `/api/kemenkes/tempat-tidur` | Tambah TT baru ke Kemenkes |
| `PUT` | `/api/kemenkes/tempat-tidur/{id_tt}` | Update TT di Kemenkes |

---

## 10. Testing

### Jalankan Semua Test

```bash
go test ./...
```

### Dengan Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

> **Catatan:** Flag `-race` tidak bisa digunakan di Windows karena `CGO_ENABLED=0`.

### Integration Test (Repository)

```bash
TEST_DATABASE_DSN="server=...;port=...;user id=...;password=...;database=..." go test ./internal/repository/... -v -run Integration
```

---

## 11. Konvensi Git

Proyek ini menggunakan **Conventional Commits**.

Format:
```
<type>(<scope>): <subject>
```

**Tipe:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`, `revert`

**Scope:** `config`, `handler`, `worker`, `repo`, `ui`, `logger`, `main`

Contoh commit:
```bash
feat(worker): add exponential backoff on API timeout
fix(repo): prevent nil pointer in GetBedAvailability
docs(readme): update API endpoint table
```

Aktifkan template commit:
```bash
git config commit.template .gitmessage
```

---

## 12. Dokumentasi Lengkap

Dokumentasi teknis lengkap tersedia:

- [📘 DOKUMENTASI.md](DOKUMENTASI.md) — Panduan instalasi, arsitektur, dan penggunaan UI
- [🏗 architecture.md](architecture.md) — Arsitektur sistem dan task list
- [🛏 architecture-input-sk-bed.md](architecture-input-sk-bed.md) — Detail fitur input SK dan pemetaan bed
- [📊 comprehensive_review.md](comprehensive_review.md) — Review code dan finding

---

## Lisensi

Proprietary — **RSUD Sleman**. Seluruh hak cipta dilindungi.