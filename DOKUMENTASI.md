# Dokumentasi Aplikasi SIRS Online Bridging V3
## RSUD Sleman

Dokumentasi ini menjelaskan proses pengembangan aplikasi dari tahap perancangan hingga menjadi aplikasi berbasis web, detail implementasi, serta panduan penggunaan.

---

## 1. Langkah Pengembangan (Step-by-Step)

Proses pengembangan aplikasi ini dibagi menjadi beberapa fase utama:

### Fase 1: Perancangan Arsitektur (`architecture.md`)
Ini adalah titik awal aplikasi di mana struktur dasar dan alur kerja didefinisikan:
- **Stack Teknologi**: Memilih Golang untuk backend (performa tinggi & concurrency), SQL Server untuk database SIMRS, dan Alpine.js + Tailwind CSS untuk dashboard.
- **Konsep Worker Pool**: Merancang sistem yang berjalan di background setiap 2 jam untuk melakukan sinkronisasi otomatis.
- **Mapping API**: Menganalisis endpoint Kemenkes (GET, POST, PUT) dan struktur JSON yang dibutuhkan.

### Fase 2: Infrastruktur & Konfigurasi
- **Inisialisasi Proyek**: Pengaturan `go.mod` dan instalasi dependencies seperti `viper` (konfigurasi), `go-mssqldb` (SQL Server), dan `resty` (HTTP Client).
- **Manajemen Konfigurasi**: Implementasi `config/config.go` yang membaca file `.env` untuk fleksibilitas pengaturan tanpa mengubah kode.
- **Sistem Logging**: Pembuatan logger kustom di `internal/logger` untuk mencatat aktivitas ke file `.log` dan konsol secara simultan.

### Fase 3: Implementasi Core (Backend)
- **Layer Database (Repository)**: Membuat `bed_repository.go`. Hal kritikal di sini adalah penggunaan **Database Transaction (`*sql.Tx`)** karena query menggunakan table temporary `#temp_ranap` yang hanya hidup dalam satu sesi koneksi.
- **Otomatisasi (Worker Pool)**: Implementasi `Dispatcher` dan `Worker`. Dispatcher mengatur *ticker* (penjadwal), sedangkan Worker mengeksekusi logika pengambilan data dari SIMRS dan pengiriman ke API Kemenkes.

### Fase 4: Antarmuka Web (Dashboard)
- **REST API Internal**: Membangun endpoint seperti `/api/beds` untuk data real-time, `/api/logs` untuk monitoring, dan `/api/sync` untuk pemicu manual.
- **API Proxy**: Implementasi handler khusus di `main.go` yang meneruskan request ke Kemenkes dengan menambahkan header wajib (`X-rs-id`, `X-pass`, `X-Timestamp`) secara otomatis.
- **Frontend Dashboard**: Menggunakan `index.html` dengan Tailwind CSS untuk tampilan premium dan Alpine.js untuk logika tab serta fetch data dinamis.

### Fase 5: Deployment (Windows Service)
- Menambahkan *wrapper* menggunakan `golang.org/x/sys/windows/svc` agar aplikasi bisa berjalan sebagai layanan Windows. Ini memungkinkan aplikasi berjalan otomatis saat server *booting* tanpa harus ada user yang login.

---

## 2. Detail Implementasi Teknis

### Struktur Proyek
```
/
├── config/             # Manajemen konfigurasi (.env)
├── internal/
│   ├── handler/        # Controller untuk REST API
│   ├── logger/         # Sistem pencatatan aktivitas
│   ├── repository/     # Logika database (SQL Server)
│   └── worker/         # Penjadwal & eksekutor sinkronisasi
├── logs/               # Lokasi file .log
├── web/static/         # Frontend (HTML, Tailwind, Alpine)
├── .env                # Pengaturan sensitif
├── architecture.md     # Cetak biru aplikasi
└── main.go             # Entry point aplikasi
```

### Logika Sinkronisasi Data
1. **Deteksi SK**: Sistem mencari Nomor SK yang masih aktif (tanggal berakhir NULL).
2. **Kalkulasi Bed**: Menggunakan SQL query untuk menghitung total bed di `sk_bed` dan membandingkannya dengan pasien aktif di `#temp_ranap`.
3. **API PUT**: Data dikirim baris demi baris ke endpoint `Fasyankes/{id_tt}` milik Kemenkes.

---

## 3. Panduan Penggunaan

### Persiapan Konfigurasi (`.env`)
Salin atau buat file `.env` di direktori utama dengan format:
```env
# Database SIMRS
DB_HOST=192.168.x.x
DB_PORT=1433
DB_USER=root
DB_PASS=password_anda
DB_NAME=nama_database

# API RS Online
API_URL=https://sirs.kemkes.go.id/fo/index.php/Fasyankes
API_RS_ID=kode_rs_anda
API_PASS=password_api_anda

# Operasional
APP_PORT=9271
SYNC_INTERVAL_HOURS=2
LOG_FILE=logs/sirs.log
```

### Prasyarat & Instalasi
Sebelum menjalankan aplikasi, pastikan Anda telah menginstal **Golang (versi 1.23 atau lebih baru)**.

Aplikasi ini menggunakan beberapa library pihak ketiga. Jalankan perintah berikut di terminal/command prompt pada direktori proyek untuk mengunduh semua dependensi yang diperlukan sesuai dengan `go.mod`:

```bash
go mod tidy
```

Library utama yang akan diinstal meliputi:
- `resty/v2`: Untuk komunikasi dengan API Kemenkes.
- `go-mssqldb`: Driver untuk koneksi ke SQL Server.
- `viper`: Untuk manajemen konfigurasi melalui file `.env`.
- `golang.org/x/sys`: Untuk integrasi Windows Service.

### Cara Menjalankan
1. **Mode Pengembangan**:
   Jalankan perintah: `go run main.go`
   Aplikasi akan berjalan di mode interaktif (konsol).
2. **Mode Produksi (Windows Service)**:
   - Build aplikasi: `go build -o sirs-online.exe`
   - Daftar sebagai service: `sc create sirs-online binPath= "C:\path\ke\sirs-online.exe"`
   - Jalankan service: `sc start sirs-online`

### Menggunakan Dashboard
Akses dashboard melalui browser di: `http://localhost:9271`

- **Tab 1 — Info Ruang**: Melihat data ketersediaan bed saat ini hasil perhitungan SIMRS.
- **Tab 2 — Master Referensi**: Melihat daftar jenis tempat tidur resmi dari Kemenkes.
- **Tab 3 — Manajemen TT**: Mengirim data baru (POST) atau memperbarui data secara manual (PUT).
- **Tab 4 — Operasional**: 
    - Melihat log aktivitas terakhir.
    - Menjalankan sinkronisasi secara manual dengan tombol **"Sync Now"**.
    - Memantau status worker (IDLE/RUNNING) dan SK aktif.
