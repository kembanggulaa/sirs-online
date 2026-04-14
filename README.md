# SIRS Online Bridging System V3

> Sistem otomatis untuk mensinkronisasi data ketersediaan tempat tidur dari **SIMRS (SQL Server)** ke **API RS Online Kemenkes**.

---

## 📋 Daftar Isi

1. [Gambaran Umum](#-gambaran-umum)
2. [Fitur Utama](#-fitur-utama)
3. [Teknologi yang Digunakan](#-teknologi-yang-digunakan)
4. [Arsitektur Sistem](#-arsitektur-sistem)
5. [Prasyarat](#-prasyarat)
6. [Instalasi & Konfigurasi](#-instalasi--konfigurasi)
7. [Menjalankan Aplikasi](#-menjalankan-aplikasi)
8. [Endpoint API](#-endpoint-api)
9. [Testing](#-testing)
10. [Konvensi Git](#-konvensi-git)
11. [Dokumentasi Lengkap](#-dokumentasi-lengkap)
12. [Lisensi](#-lisensi)

---

## 🚀 Gambaran Umum

SIRS Online Bridging V3 adalah layanan latar belakang (*background service*) yang dibangun dengan Go untuk **RSUD Sleman**. Sistem ini berfungsi sebagai jembatan antara database SIMRS internal dan API publik Kemenkes, melaporan ketersediaan tempat tidur secara otomatis setiap interval yang dapat dikonfigurasi.

Aplikasi ini menyertakan **dashboard web** berbasis browser untuk pemantauan real-time, manajemen SK (Surat Keterangan), pemetaan tempat tidur, serta sinkronisasi manual.

---

## ✨ Fitur Utama

| Fitur | Deskripsi |
|---|---|
| **Sinkronisasi Otomatis** | Worker pool terjadwal yang berjalan setiap N jam (default: 2 jam) |
| **Graceful Shutdown** | Mendukung sinyal OS (`SIGTERM`, `SIGINT`) dan perintah stop Windows Service |
| **Dashboard Web** | Antarmuka berbasis browser (Alpine.js + Tailwind CSS) untuk monitoring dan kontrol manual |
| **Manajemen SK** | Tampil daftar SK, lihat detail, preview, dan import data SK-Bed dari UI |
| **Pemetaan Tempat Tidur** | Manajemen pemetaan `class_room_id` ke data Kemenkes langsung dari dashboard |
| **Proxy Kemenkes** | Endpoint proxy read-only dan write ke API Kemenkes (referensi TT, Fasyankes) |
| **Windows Service** | Dapat dideploy sebagai Windows Service persisten menggunakan `sc.exe` |
| **Structured Logging** | File log berputar dengan 200 baris terakhir bisa dilihat via API |
| **Keamanan** | Header CORS yang dikonfigurasi, batas ukuran request body, TLS skip verify yang bisa diatur |

---

## 🛠 Teknologi yang Digunakan

| Komponen | Teknologi | Versi |
|---|---|---|
| **Backend** | Go | `1.23.0` |
| **Database** | Microsoft SQL Server | - |
| **Driver DB** | `github.com/microsoft/go-mssqldb` | `v1.9.2` |
| **HTTP Client** | `github.com/go-resty/resty/v2` | `v2.12.0` |
| **Konfigurasi** | `github.com/spf13/viper` | `v1.18.2` |
| **Windows Service** | `golang.org/x/sys` | `v0.33.0` |
| **Frontend** | Alpine.js + Tailwind CSS | (embedded di `web/static/`) |

---

## 🏗 Arsitektur Sistem

```
sirs-online/
├── main.go                     # Entry point & bootstrap (console / Windows Service)
├── config/
│   └── config.go               # Konfigurasi nested: Database, API, Operational, Security
├── internal/
│   ├── handler/
│   │   ├── api_handler.go      # Endpoint internal: beds, logs, sync, worker status, healthz
│   │   ├── beds_handler.go     # Endpoint manajemen tempat tidur (rooms, kamar, upsert)
│   │   ├── sk_handler.go       # Endpoint manajemen SK (list, detail, preview, import)
│   │   └── proxy_handler.go    # Endpoint proxy ke API Kemenkes (referensi, fasyankes)
│   ├── worker/
│   │   ├── dispatcher.go       # Penjadwal ticker & trigger manual
│   │   ├── worker.go           # Logika sinkronisasi (SK deteksi → ambil bed → PUT Kemenkes)
│   │   └── client.go           # HTTP client Kemenkes (Resty + TLS config)
│   ├── repository/
│   │   ├── interfaces.go       # Interface: BedRepositoryInterface, SKRepositoryInterface, BedsRepositoryInterface
│   │   ├── bed_repository.go   # Implementasi untuk Worker (GetActiveSKNo, GetBedAvailability)
│   │   ├── beds_repository.go  # Implementasi untuk BedsHandler (rooms, kamar, upsert)
│   │   └── sk_repository.go    # Implementasi untuk SKHandler (list, detail, bulk insert)
│   └── logger/
│       └── logger.go           # Logger ke file dengan ReadLast() untuk API /logs
└── web/
    └── static/                 # File statis dashboard (HTML, JS, CSS)
```

### Alur Sinkronisasi

```
[Ticker / Manual Trigger]
        │
        ▼
  Dispatcher.dispatch()
        │
        ▼
  Worker.processJob()
   ├── 1. GetActiveSKNo()         ← Query ke SIMRS
   ├── 2. GetBedAvailability()    ← Query ke SIMRS (CTE, tanpa temp table)
   ├── 3. GET /Fasyankes          ← Fetch mapping id_t_tt dari Kemenkes
   └── 4. PUT /Fasyankes          ← Per ruangan, dengan retry (max RetryMax+1)
```

---

## 📦 Prasyarat

- [Go 1.23+](https://golang.org/dl/)
- Akses ke database **SQL Server SIMRS** (host, port, user, password, database)
- Kredensial **API RS Online Kemenkes** (`API_RS_ID` dan `API_PASS`)
- Sistem Operasi: **Windows** (untuk mode Windows Service)

---

## ⚙️ Instalasi & Konfigurasi

### 1. Clone & Install Dependencies

```bash
git clone <repository-url>
cd sirs-online
go mod tidy
```

### 2. Buat File `.env`

Salin file contoh dan isi dengan nilai yang sebenarnya:

```bash
copy .env.example .env
```

Kemudian edit file `.env`:

```env
# ── Database SIMRS (SQL Server) ──────────────────────────────────────
DB_HOST=localhost
DB_PORT=1433
DB_USER=nama_user
DB_PASS=password
DB_NAME=nama_database

# ── API RS Online Kemenkes ────────────────────────────────────────────
API_URL=https://sirs.kemkes.go.id/fo/index.php
API_RS_ID=KODE_RS_ANDA
API_PASS=GANTI_DENGAN_API_PASS

# ── Operasional ───────────────────────────────────────────────────────
APP_PORT=9271
SYNC_INTERVAL_HOURS=2
RETRY_MAX=2
LOG_FILE=logs/sirs.log

# ── Monitoring Internal (Dashboard Eksekutif) ─────────────────────────
EXECUTIVE_API_URL=API-URL-MONITORING

# ── TLS (untuk API pemerintah dengan sertifikat bermasalah) ──────────
TLS_SKIP_VERIFY=false

# ── Kode RS untuk field org_unit_code ────────────────────────────────
ORG_UNIT_CODE=KODE_RS_ANDA

# ── Keamanan ──────────────────────────────────────────────────────────
DASHBOARD_ORIGIN=http://localhost:9271
MAX_BODY_BYTES=1048576
```

### Referensi Variabel Konfigurasi

| Variabel | Tipe | Default | Keterangan |
|---|---|---|---|
| `DB_HOST` | string | - | Host SQL Server |
| `DB_PORT` | int | `1433` | Port SQL Server |
| `DB_USER` | string | - | Username database |
| `DB_PASS` | string | - | Password database |
| `DB_NAME` | string | - | Nama database SIMRS |
| `API_URL` | string | - | Base URL API Kemenkes |
| `API_RS_ID` | string | - | Kode RS untuk header `X-rs-id` |
| `API_PASS` | string | - | Password API Kemenkes |
| `APP_PORT` | int | `9271` | Port HTTP server dashboard |
| `SYNC_INTERVAL_HOURS` | int | `2` | Interval sinkronisasi otomatis (jam) |
| `RETRY_MAX` | int | `2` | Jumlah retry jika PUT gagal |
| `LOG_FILE` | string | `logs/sirs.log` | Path file log |
| `TLS_SKIP_VERIFY` | bool | `false` | Skip verifikasi TLS (hanya jika diperlukan) |
| `ORG_UNIT_CODE` | string | - | Kode RS untuk `org_unit_code` |
| `DASHBOARD_ORIGIN` | string | - | Origin CORS dashboard (misal: `http://localhost:9271`) |
| `MAX_BODY_BYTES` | int64 | `1048576` | Batas ukuran body request POST/PUT (bytes) |
| `EXECUTIVE_API_URL` | string | - | URL API dashboard eksekutif |

---

## ▶️ Menjalankan Aplikasi

### Mode Development (Console)

```bash
go run main.go
```

Dashboard akan dapat diakses di: `http://localhost:9271`

### Build Binary

```bash
go build -o sirs-online.exe .
./sirs-online.exe
```

### Mode Windows Service (Produksi)

```bash
# Daftarkan sebagai Windows Service (jalankan sebagai Administrator)
sc create SIRSOnline binPath="C:\path\to\sirs-online.exe" start=auto

# Jalankan service
sc start SIRSOnline

# Hentikan service
sc stop SIRSOnline

# Hapus service
sc delete SIRSOnline
```

> **Catatan:** Saat berjalan sebagai Windows Service, aplikasi mendeteksi mode otomatis menggunakan `svc.IsAnInteractiveSession()`.

---

## 🔌 Endpoint API

Semua endpoint berjalan di `http://localhost:{APP_PORT}`.

### Dashboard & Static Files

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/` | Halaman dashboard utama (static files dari `web/static/`) |

### Sinkronisasi & Monitoring (APIHandler)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/beds` | Ambil data ketersediaan bed dari in-memory state |
| `GET` | `/api/logs` | Ambil 200 baris terakhir dari file log |
| `POST` | `/api/sync` | Trigger sinkronisasi manual |
| `GET` | `/api/worker/status` | Status worker: `Running` atau `Idle` |
| `GET` | `/api/sk-active` | SK aktif yang sedang digunakan worker |
| `GET` | `/api/healthz` | Health check endpoint |

### Manajemen SK (SKHandler)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/sk/list` | Daftar semua nomor SK |
| `GET` | `/api/sk/detail?sk_no=<SK>` | Detail baris SK tertentu |
| `POST` | `/api/sk/preview` | Preview data sebelum import (validasi saja) |
| `POST` | `/api/sk/import` | Import/bulk insert data SK-Bed ke database |

### Manajemen Tempat Tidur (BedsHandler)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/beds/rooms` | Daftar distinct `class_room_id` dari SIMRS |
| `GET` | `/api/beds/kamar?class_room_id=<ID>` | Daftar kamar berdasarkan class_room_id |
| `GET` | `/api/beds/by-room?class_room_id=<ID>` | Data beds lengkap per ruangan |
| `POST` | `/api/beds/upsert` | Upsert mapping beds untuk class_room_id tertentu |

### Proxy Kemenkes (ProxyHandler)

| Method | Path | Keterangan |
|---|---|---|
| `GET` | `/api/proxy/referensi` | Ambil referensi tempat tidur dari Kemenkes |
| `GET` | `/api/proxy/fasyankes` | Ambil data Fasyankes yang sudah diinput RS |
| `GET` | `/api/beds/executive` | Data dashboard eksekutif (proxy ke Executive API) |
| `POST` | `/api/kemenkes/tempat-tidur` | Tambah tempat tidur baru ke Kemenkes |
| `PUT` | `/api/kemenkes/tempat-tidur/{id_tt}` | Update tempat tidur di Kemenkes |

---

## 🧪 Testing

### Jalankan Semua Test

```bash
go test ./...
```

### Dengan Coverage Report

```bash
go test -coverprofile=coverage.out ./...
go tool cover -html=coverage.out
```

### Ringkasan Coverage Saat Ini

| Package | Coverage |
|---|---|
| `config` | **100.0%** |
| `internal/worker` | **69.9%** |
| `internal/logger` | **62.7%** |
| `internal/handler` | **45.1%** |

> Unit test menggunakan **mock interface** (`BedRepositoryInterface`, `SKRepositoryInterface`, `BedsRepositoryInterface`) untuk menghindari ketergantungan pada database nyata.

---

## 📝 Konvensi Git

Proyek ini menggunakan **Conventional Commits** dengan template di `.gitmessage`.

```
<type>(<scope>): <subject>
```

**Tipe yang valid:** `feat`, `fix`, `docs`, `style`, `refactor`, `test`, `chore`, `perf`, `ci`, `revert`

**Scope yang valid:** `config`, `handler`, `worker`, `repo`, `ui`, `logger`, `main`

**Contoh commit:**
```bash
feat(worker): add exponential backoff on API timeout
fix(repo): prevent nil pointer in GetBedAvailability
refactor(config): extract security config into nested struct
docs(readme): update API endpoint table
```

**Aktifkan template commit:**
```bash
git config commit.template .gitmessage
```

---

## 📖 Dokumentasi Lengkap

Dokumentasi teknis lengkap tersedia dalam Bahasa Indonesia:

- [📘 DOKUMENTASI.md](DOKUMENTASI.md) — Panduan instalasi, arsitektur, dan penggunaan UI secara lengkap
- [🏗 architecture.md](architecture.md) — Arsitektur sistem dan task list
- [🛏 architecture-input-sk-bed.md](architecture-input-sk-bed.md) — Detail arsitektur fitur input SK dan pemetaan bed

---

## 📜 Lisensi

Proprietary — **RSUD Sleman**. Seluruh hak cipta dilindungi.
