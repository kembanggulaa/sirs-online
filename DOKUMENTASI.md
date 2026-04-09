# SIRS Online Bridging V3

## Daftar Isi
1. Ringkasan Sistem (Overview)
2. Persyaratan Sistem (System Requirements)
3. Instalasi & Konfigurasi (Installation & Configuration)
4. Menjalankan Aplikasi (Running the Application)
5. Arsitektur Sistem (System Architecture)
6. Dokumentasi API (API Documentation)
7. Panduan Penggunaan UI (UI Usage Guide)
8. Pemecahan Masalah (Troubleshooting)

## 1. Ringkasan Sistem (Overview)
SIRS Online Bridging V3 adalah aplikasi perantara (bridging) yang menghubungkan Sistem Informasi Manajemen Rumah Sakit (SIMRS) berbasis Microsoft SQL Server dengan API SIRS Online Kementerian Kesehatan Republik Indonesia. Aplikasi ini menyediakan mekanisme sinkronisasi data ketersediaan tempat tidur (bed) otomatis yang berjalan di latar belakang (worker background process), sebuah *proxy server* untuk mengakses endpoint Kemenkes secara aman (menghindari kendala CORS), serta dashboard berbasis antarmuka web interaktif untuk memonitor status sinkronisasi, log aktivitas, pemetaan bed, dan manajemen SK terkait ranap.

## 2. Persyaratan Sistem (System Requirements)
- **Sistem Operasi**: Windows (dirancang langsung mendukung Windows Service API maupun Console App interaktif).
- **Database**: Microsoft SQL Server (Mssql).
- **Bahasa Pemrograman**: Golang (v1.23.0 atau yang lebih baru).
- **Dependensi Utama**: 
  - `github.com/microsoft/go-mssqldb` (Driver konektor database mssql)
  - `github.com/go-resty/resty/v2` (HTTP Client untuk memanggil Web Services API Kemenkes)
  - `github.com/spf13/viper` (Manajemen Load Data Konfigurasi/Env)
  - `golang.org/x/sys` (Library sistem core OS, terkhusus Windows service integration)

## 3. Instalasi & Konfigurasi (Installation & Configuration)

1. **Persiapan Repositori**:
   ```bash
   git clone <url-repo-anda>
   cd sirs-online
   ```

2. **Instalasi Dependencies Go**:
   ```bash
   go mod tidy
   ```

3. **Konfigurasi Lingkungan (`.env`)**:
   Gandakan dan ubah nama dari file `.env.example` menjadi `.env`.
   ```bash
   cp .env.example .env
   ```
   Buka file `.env` di teks editor pilihan, dan konfigurasi sesuai dengan kondisi riil Rumah Sakit Anda:
   ```env
   # Setup Koneksi Database SIMRS
   DB_HOST=127.0.0.1
   DB_PORT=1433
   DB_USER=sa
   DB_PASS=P@ssw0rdDB
   DB_NAME=db_simrs_utama
   
   # Setup Kredensial SIRS Online Kemenkes (Disesuaikan dengan YANKES)
   API_URL=https://sirs.kemkes.go.id/fo/index.php
   API_RS_ID=KODE_RS_ANDA
   API_PASS=PASSWORD_RAHASIA_API_ANDA

   # Operasional Internal Ticker / Bridge Server
   APP_PORT=9271
   SYNC_INTERVAL_HOURS=2
   LOG_FILE=logs/sirs.log
   ORG_UNIT_CODE=KODE_RS_ANDA
   DASHBOARD_ORIGIN=http://localhost:9271
   TLS_SKIP_VERIFY=false
   ```

## 4. Menjalankan Aplikasi (Running the Application)

Aplikasi Golang ini memiliki fungsionalitas unik yang membedakan penanganan ketika dijalankan melalui Terminal Console maupun sebagai Background Windows Service Process (`sirsService`).

**A. Mode Pengembangan / Interaktif (Console)**:
Bisa dipanggil langsung untuk keperluan pengamatan secara real-time.
```bash
go run main.go
# Atau jika executable binary telah di build (go build -o sirs-online.exe):
./sirs-online.exe
```

**B. Pengaturan Modus Produksi**:
Untuk kebutuhan 24-jam non-stop, pengguna dapat mendaftartakan service ini lewat Windows Service Manager bawaan OS.
Buka `http://localhost:9271` pada penelusuran web peramban (Chrome/Firefox/Edge) untuk memastikan dashboard muncul dan backend telah melayani request ke port `9271`.

## 5. Arsitektur Sistem (System Architecture)
Aplikasi dibangun menggunakan pola rancangan modular di dalam direktori `internal/` (Sesuai standard Go idiom layout struct):
- **Handler (`internal/handler`)**: 
  - `APIHandler`: Mengurus rute yang mengekspos endpoint internal (cek status logger, memicu sinkronisasi dispatcher secara paksa (manual override), metrik utilitas program).
  - `ProxyHandler`: Merutekan/menyuntikkan kredensial rahasia (ID RS/Pass) yang ada dalam `config` kepada peramban yang merequest referensi TT, meneruskan (`proxying`) traffic client ke web service `Kemenkes` tanpa harus menulis data kredensial mentah (Hardcode) pada JavaScript Front-End (`web/static/js/*`).
  - `SKHandler` & `BedsHandler`: Menangani pengurusan modul SK Aktif serta Manajemen Pemetaan ID Bed secara komprehensif.
- **Worker (`internal/worker`)**: Merupakan urat nadi penghubung dan pembaruan massal (bulk synchronization). Aktif dan bekerja pada mode interval ter-setup lewat `SYNC_INTERVAL_HOURS` variabel.
- **Repository (`internal/repository`)**: Kumpulan abstraksi kueri Transact-SQL (T-SQL / SQL Server) diubah ke bentuk API-friendly. Semua akses modifikasi `Read-Write` dilakukan pada level lapis ini.
- **Config & Logger**: Lapisan utilitas pengurai berkas inisiasi awal saat program terbit (`booting`) serta mencatat `Info`, `Error`, dan kejadian malfungsi di penyimpanan file statis.

## 6. Dokumentasi API (API Documentation)

### Internal Backend Dashboard API
- `GET /api/beds`: Mengambil data list kamar beserta real-time statistik pemakaiannya hari ini in-memory dispatcher worker.
- `GET /api/logs`: Menampilkan log sistem terbaru (kapasitas pengembalian maks: 200 baris terakhir dari *log* aplikasi).
- `POST /api/sync`: Memicu jalannya proses sinkronisasi worker secar instan untuk saat itu juga. (Mengabaikan jam interval antrian berjalan).
- `GET /api/worker/status`: Mengembalikan respon Status saat ini atas kondisi utilitas dispatcher worker (`Idle` / tertidur vs `Running` / sedang push ke kemenkes).
- `GET /api/sk-active`: Pengambilan konfirmasi string text atas referensi nomor SK ranap RS saat ini.
- `GET /api/healthz`: Tanda detak server bagi keperluan load balancing/up-time status.

### Bridge Proxy Endpoints (Layanan Kemenkes)
- `GET /api/proxy/referensi`: Meneruskan permintaan ambil list Referensi TT dari sub URL `/Referensi/tempat_tidur` Kemenkes.
- `GET /api/proxy/fasyankes`: Melihat catatan jumlah sinkronisasi fasilitas pada sisi database sistem pusat SIRS Kemenkes saat pengecekan berjalan.
- `POST /api/kemenkes/tempat-tidur`: Endpoint Post baru ke modul `/Fasyankes` di website Pusat.
- `PUT /api/kemenkes/tempat-tidur/{id_tt}`: Update form kapasitas tempat tidur menuju modul `/Fasyankes`.
- `GET /api/beds/executive`: Terusan proxy request ke custom `EXECUTIVE_API_URL` internal rumah sakit.

### Manajemen Tempat Tidur (Ranap & Master Data & SK)
- `GET /api/beds/rooms`: Mendapat ringkasan daftar relasi ID Ruangan (Class Room IDs).
- `GET /api/beds/kamar?class_room_id=...`: Menampilkan kelompok kamar spesifik terkait ID Ruangan / *Class Room*.
- `GET /api/beds/by-room?class_room_id=...`: Menagih daftar list `raw` beds/ranap dalam sekat scope ruangan terkait.
- `POST /api/beds/upsert`: Melakukan mutasi (Update ada/Insert tambah) deretan data relasi bed dari perbaikan mapping form ke server lokal database `mssql`.
- Endpoint `SK`: Membuka jalan akses integratif yang berpusat pada pembaruan SK operasional ranap terbaru (`/api/sk/list`, `/api/sk/detail`, pemaparan pra-tampil table unggahan `/api/sk/preview`, mengeksekusinya ke penyimpanan `/api/sk/import`).

## 7. Panduan Penggunaan UI (UI Usage Guide)

Setelah aplikasi dijalankan, navigasikan layar Web Browser ke `http://localhost:9271/`. Sistem ini membawa antarmuka `web/static/index.html` dengan pembagian fungsi utamanya sebagai berikut layaknya petunjuk di *Onboarding Modal*:

- **Tab 1 — Info Ruang**: Memantau secara *real-time* ketersediaan bed di rumah sakit hasil kalkulasi DB SIMRS. Indikator bar *Occupancy* di layar akan berubah menjadi warna hijau, kuning, atau merah tergantung persentase keterisiannya.
- **Tab 2 & Tab 3 — Master Referensi & Fasyankes**:
  - *Master Referensi*: Menampilkan katalog riwayat tempat tidur (TT) remis rujukan dari sisi Kemenkes.
  - *Master Fasyankes*: Adalah data rumah sakit kita yang sudah pernah diinputkan ke sistem layanan Kemenkes. Anda bisa menekan tombol **"Gunakan PUT"** untuk menyalin data lama tersebut guna diperbarui (*edit*).
- **Tab 4 — Manajemen TT**: Digunakan jika Anda ingin (*nge-push*) mengirim data ruang masuk secara manual (seperti kasus perbaikan darurat). Gunakan form isian POST untuk membuat data baru di server Kemenkes, atau form PUT untuk *update* kapasitas ruang yang sudah ada.
- **Tab 5 — Manajemen SK**: Digunakan pada saat turun rilis kebijakan SK Tempat Tidur baru. Alurnya mencakup 3 step operasional:
  1. *Step 1 (Header)*: Isi Nomor SK baru dan Tanggal mulainya, operator rekam medis juga bisa klik **Unggah Excel** untuk jalur cepat masal.
  2. *Step 2 (Input Data)*: Mengisikan data bangsal per baris. Ketentuan pengisian **wajib** tepat persis format Kemenkes (terdapat 15 kolom kunci standar `clinic_id`, `class_room_id`, dll).
  3. *Step 3 (Preview)*: Cek kembali secara ringkas. Dengan tahapan ini dikonfirmasi/disimpan, maka SK edisi lama akan "dilengserkan"/"dipensiunkan" secara otomatis beralih ke SK Baru tersebut.
- **Tab 6 — Manajemen Beds**: Setelah SK berhasil diterbitkan, maka RS **WAJIB** memetakan tiap titik nomor bed per ruangannya di sini.
  1. *Pilih Bangsal*: Mulai dengan memilih bangsal yang tersaji dinamis dari form SK di langkah sebelumnya.
  2. *Manajemen Kamar*: Lalu buat kelompok blok kamar dengan tombol biru "Tambah Kamar Baru" (Misal: menamainya blok "Kamar VIP 1").
  3. *Manajemen Detail Baris Bed*: Klik pada "+ Tambah Bed". Masukkan detail rijit yang diperlukan utamanya **bed_id**. Bila bed itu dispesifikkan untuk pasien Suspek/Konfirmasi COVID, anda wajib mencentangnya `(Ya)` pada kotak COVID-19.
  4. Akhiri dengan klik utamakan aksi **Simpan Data Beds**.
- **Tab Operasional**: Tempat Anda mengelola nafas *"Nyawa"* integrasi sistem di belakang layar. Anda dapat melihat rekapan *Viewer Log* pengiriman paket XML/JSON, mengecek detak status *dispatcher* (Status terlihat *Running* atau *Idle*), serta bila dirasa perlu Anda dipersilakan menekan tombol paksa pengiriman sekarang tanpa menunggu putaran interval sinkronisasi 2 jam ke depan dengan (**Trigger Sync Now/Sync Sekarang**).
- **Dashboard Eksekutif Tambahan (`eksekutif.html`)**: Menu navigasi mandiri khusus untuk pimpinan dan level pengambil keputusan yang hanya ingin melongok rangkuman agregat ketersediaan Rawat Inap (Ranap) seketika tanpa pusing menyentuh panel operasional di atasnya.

## 8. Pemecahan Masalah (Troubleshooting)

1. **Dashboard Kosong Atau "404 Not Found"**  
   ⚠️ *Penyebab & Solusi*: Hal ini disebabkan path root yang salah (misalkan di jalankan dari beda *parent folder*). Aplikasi sangat bergantung keberadaan folder `web/static/` di folder program aplikasi di eksekusi. Konfirmasikan path terminal sewaktu `go run main.go` atau pendaftaran run di OS Windows mengarah pada Working Directory program.
2. **Koneksi Database MSSQL Gagal Menyatu**  
   ⚠️ *Penyebab & Solusi*: `DB_PORT`, `DB_HOST`, tipe nama instance (misal menggunakan backslash nama komputer `SQLSERVER\SQLEXPRESS`) tidak terbaca. Pastikan user sa tidak terlockout dan port SQL Browser bisa dicapai program `sirs-online`.
3. **Pesan SSL Certificate Verification Error Saat Hit Kemenkes API**  
   💡 *Penyebab & Solusi*: Platform API Pemerintah kadang mengalami latensi perpindahan rotasi Sertifikat Keamanan SSL. Jika traffic menolak sertifikat tersebut, anda dapat merelaksasi batas cek sertifikat menggunakan parameter `.env` dan beri format: `TLS_SKIP_VERIFY=true`.
4. **Log tidak tersimpan / Cannot Open Error File Permissions**  
   💡 *Penyebab & Solusi*: Folder path tidak tersedia terlebih dulu. Pastikan folder `logs/` telah termuat kosong di root direktori program, serta OS user dari Windows Service yang berjalan memiliki kapabilitas / izin perihal *Read & Write* file padanya.
