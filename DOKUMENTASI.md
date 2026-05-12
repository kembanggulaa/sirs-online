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
  - `github.com/gin-gonic/gin` (HTTP framework - migrasi dari net/http pada v2.0.0)
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

**HTTP Framework**: Aplikasi menggunakan **Gin** (`github.com/gin-gonic/gin`) sejak v2.0.0 untuk routing dan middleware. Sebelumnya menggunakan `net/http` standard library.

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

## 9. Adaptasi untuk Rumah Sakit Lain (Porting Guide)

Aplikasi ini dirancang spesifik untuk SIMRS dengan schema SQL Server seperti yang digunakan di RSUD Sleman. Jika rumah sakit lain ingin menggunakan aplikasi ini, beberapa bagian kode **harus diubah** sesuai dengan schema SIMRS mereka. Bagian yang paling besar perubahannya adalah query-query di repository.

### 9.1 Database yang Didukung

Kode ini **hanya mendukung SQL Server**. Jika Anda menggunakan PostgreSQL atau MySQL, Anda harus:
1. Mengganti driver di `go.mod` (dari `github.com/microsoft/go-mssqldb` ke `github.com/lib/pq` atau `go-sql-driver/mysql`)
2. Mengubah semua query T-SQL ke syntax database yang Anda gunakan
3. Mengubah koneksi database di `main.go` dan `config/config.go`

**Sintaks T-SQL yang tidak portable:**
- `WITH (NOLOCK)` — hint tabel SQL Server. Di PostgreSQL/MySQL: hapus atau ganti dengan `FOR UPDATE`
- `IIF(cond, true_val, false_val)` — fungsi IIF SQL Server. Di database lain: ganti dengan `CASE WHEN cond THEN true_val ELSE false_val END`
- `ISNULL(expr, default)` — fungsi null-check SQL Server. Di database lain: ganti dengan `COALESCE(expr, default)`
- `NULLIF(expr, '')` — sama di semua database, tapi perlu hati-hati saat dikombinasikan dengan `LTRIM(RTRIM(...))`
- CTE `WITH TempRanap AS (...)` — secara syntax ANSI SQL, CTE didukung PostgreSQL dan MySQL 8+. Namun logic SQL Server session-bound temp table (`#temp`) tidak tersedia di database lain

### 9.2 Langkah Perubahan per Repository

#### A. `internal/repository/bed_repository.go`

**1) `GetActiveSKNo()` — baris 40**

Query mencari SK aktif berdasarkan `tgl_berakhir IS NULL`:
```sql
-- SQL Server (asli):
SELECT DISTINCT sk_no FROM sk_bed WHERE tgl_berakhir IS NULL
```
**Yang perlu diubah:** Pastikan tabel `sk_bed` ada di SIMRS Anda, dan kolom `sk_no` serta `tgl_berakhir` namanya sesuai. Jika nama kolom beda (misal `tgl_berakhir` jadi `tanggal_berakhir`), sesuaikan.

**2) `GetBedAvailability(skNo)` — baris 68**

Ini adalah query **paling kompleks** dan **paling mungkin perlu diubah**. Query ini punya dua bagian:

**Bagian CTE `TempRanap`** (baris 71-85) — Menghitung pasien yang masih dirawat (bed terisi):
```sql
-- SQL Server (asli):
WITH TempRanap AS (
    SELECT CONCAT(b.class_room_id, b.kamar) AS kamar
    FROM pasien_visitation pv WITH (NOLOCK)
    LEFT JOIN beds b WITH (NOLOCK) ON b.class_room_id = pv.CLASS_ROOM_ID AND b.bed_id = pv.bed_id
    WHERE pv.no_registration <> ''
      AND pv.class_room_id IS NOT NULL
      AND (pv.keluar_id = 0 OR pv.keluar_id = 33)  -- belum keluar / pulang
      AND pv.class_room_id IN (
        SELECT DISTINCT class_room_id FROM sk_bed WITH (NOLOCK)
        WHERE sk_no = ? AND tgl_berakhir IS NULL AND class_room_id <> 'NI.BX'
      )
)
```
**Yang harus Anda ubah:**
- `pasien_visitation` → ganti dengan nama tabel pasien di SIMRS Anda
- `pv.CLASS_ROOM_ID` → sesuaikan dengan nama kolom Class Room ID di tabel pasien Anda
- `pv.bed_id` → sesuaikan dengan nama kolom bed ID di SIMRS Anda
- `pv.keluar_id = 0 OR pv.keluar_id = 33` → logika "belum keluar". Nilai `0` dan `33` adalah kode spesifik dari SIMRS RSUD Sleman. Rumah sakit lain mungkin punya kode berbeda (misal `status = 'active'`, atau `tanggal_keluar IS NULL`). Anda mungkin butuh kolom berbeda atau logika berbeda untuk menandai pasien masih dirawat
- `beds` → nama tabel master bed di SIMRS Anda. Jika tidak punya tabel `beds`, Anda bisa menghilangkan JOIN ke `beds` dan cukup pakai `pv.class_room_id` dan `pv.kamar` langsung

**Contoh jika pakai PostgreSQL (tanpa CTE temp table):**
```sql
-- Anda bisa inline-kan subquery langsung tanpa CTE:
SELECT
    sk.id_tt_siranap,
    sk.class_room_id,
    CASE WHEN sk.kamar IS NULL THEN sk.ruang_siranap ELSE sk.ruang_siranap || '-' || sk.kamar END AS siranap,
    ...
FROM sk_bed sk
INNER JOIN status_covid sc ON sc.id_tt = sk.id_tt_siranap
LEFT JOIN (
    SELECT class_room_id || kamar AS kamar, COUNT(*) AS terisi
    FROM pasien_visit
    WHERE keluar_id NOT IN (1, 2, 3)  -- disesuaikan
    GROUP BY class_room_id || kamar
) t ON t.kamar = sk.class_room_id || sk.kamar
WHERE ...
```

**Bagian Main Query** (baris 86-114):
```sql
-- SQL Server (asli):
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
    ISNULL(t.terisi, 0) AS terisi
FROM sk_bed sk WITH (NOLOCK)
    INNER JOIN status_covid sc WITH (NOLOCK) ON sc.id_tt = sk.id_tt_siranap
    LEFT JOIN (...) t ON t.kamar = CONCAT(sk.class_room_id, sk.kamar)
WHERE sk.sk_no = ? AND sk.tgl_berakhir IS NULL AND sk.class_room_id <> 'NI.BX'
GROUP BY ...
```
**Yang perlu diubah:**
- `sk_bed` → pastikan tabel dan kolom ini ada di SIMRS Anda. Kolom yang dipakai: `id_tt_siranap`, `class_room_id`, `kamar`, `ruang_siranap`, `jml_ruang_siranap`, `kelas_siranap`, `bed`, `covid`, `sk_no`, `tgl_berakhir`
- `status_covid` → tabel dan kolom untuk status COVID per TT (`id_tt`, `status`, `konfirmasi`, `antrian`). Jika tidak punya tabel ini, Anda perlu menghapus `INNER JOIN status_covid` dan menyesuaikan kolom yang di-SELECT
- `IIF()` → ganti dengan `CASE WHEN`
- `CONCAT()` → di PostgreSQL/MySQL bisa tetap pakai `||` atau `CONCAT()` (fungsi yang sama)
- `ISNULL(t.terisi, 0)` → ganti dengan `COALESCE(t.terisi, 0)`
- `WITH (NOLOCK)` → hapus semua hint ini untuk PostgreSQL/MySQL

#### B. `internal/repository/beds_repository.go`

**3) `GetDistinctClassRooms()` — baris 59**

Query sangat sederhana, hanya ambil daftar `class_room_id` dari `sk_bed`:
```sql
-- SQL Server (asli):
SELECT DISTINCT class_room_id FROM sk_bed WITH (NOLOCK) WHERE tgl_berakhir IS NULL ORDER BY class_room_id
```
**Yang perlu diubah:** Pastikan tabel `sk_bed` dan kolom `class_room_id` serta `tgl_berakhir` ada.

**4) `GetKamarByClassRoom(classRoomID)` — baris 84**
```sql
-- SQL Server (asli):
SELECT DISTINCT kamar FROM sk_bed WITH (NOLOCK) WHERE class_room_id = ? AND tgl_berakhir IS NULL ORDER BY kamar
```
**Yang perlu diubah:** Pastikan kolom `kamar` ada di tabel `sk_bed`. Jika nama kolom beda, sesuaikan.

**5) `GetBedsByRoom(classRoomID)` — baris 113**

Query ini punya dua SELECT:

**Query defaults** (baris 125):
```sql
-- SQL Server (asli):
SELECT ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), ISNULL(namaruang, '')) as kamar_key,
       id_tt_siranap, covid, ISNULL(kodekelas, ''), ISNULL(namakelas, '')
FROM sk_bed WITH (NOLOCK)
WHERE class_room_id = ? AND tgl_berakhir IS NULL
```
**Yang perlu diubah:**
- `ISNULL(NULLIF(LTRIM(RTRIM(kamar)), ''), namaruang)` → di PostgreSQL: `COALESCE(NULLIF(TRIM(kamar), ''), namaruang)`. Di MySQL: `COALESCE(NULLIF(TRIM(kamar), ''), namaruang)`
- Kolom yang dipakai: `kamar`, `namaruang`, `id_tt_siranap`, `covid`, `kodekelas`, `namakelas`

**Query beds** (baris 178):
```sql
-- SQL Server (asli):
SELECT bed_id, ISNULL(kamar, ''), room_id, id_kelas, nm_kelas,
       id_perawatan, nm_perawatan, id_tt_siranap, id_siranap,
       deskripsi_siranap, covid
FROM beds WITH (NOLOCK)
WHERE class_room_id = ? AND bed_id IS NOT NULL AND bed_id <> 0
ORDER BY kamar, bed_id
```
**Yang perlu diubah:** Pastikan tabel `beds` ada dengan kolom-kolom tersebut. Kolom yang WAJIB ada: `bed_id`, `kamar`, `class_room_id`. Sisanya (seperti `room_id`, `id_kelas`, `nm_kelas`, dll) mungkin perlu disesuaikan atau di-skip jika tidak ada di schema Anda.

**6) `UpsertBeds(ctx, req)` — baris 285**

Fungsi ini melakukan INSERT dan UPDATE. Perlu dicek:
- Struktur `BedsUpsertRequest` (di `interfaces.go` atau `types.go`) — sesuaikan jika kolom Anda beda
- Kolom-kolom yang di-INSERT/UPDATE: `class_room_id`, `kamar`, `bed_id`, `room_id`, `id_kelas`, `nm_kelas`, `id_perawatan`, `nm_perawatan`, `id_tt_siranap`, `id_siranap`, `deskripsi_siranap`, `covid`
- Transaksi (`BEGIN TRAN`, `COMMIT`, `ROLLBACK`) — di PostgreSQL/MySQL sintaksnya beda: `BEGIN` / `COMMIT` / `ROLLBACK` tanpa spasi

#### C. `internal/repository/sk_repository.go`

**7) `GetSKList()` — baris 157**

```sql
-- SQL Server (asli):
SELECT DISTINCT sk_no FROM sk_bed ORDER BY sk_no DESC
```
Cukup straightforward, pastikan kolom `sk_no` ada.

**8) `GetSKDetail(skNo)` — baris 180**

Ambil semua baris untuk satu SK tertentu. Pastikan kolom `sk_no` ada.

**9) `BulkInsertSKBed(ctx, req)` — baris 43**

Fungsi ini:
1. Update `tgl_berakhir` SK lama
2. Ambil `max_bed_id` untuk generate ID baru
3. INSERT batch data baru

**Yang perlu diubah:**
- `UPDATE sk_bed SET tgl_berakhir = ? WHERE sk_no = (SELECT DISTINCT sk_no FROM sk_bed WHERE tgl_berakhir IS NULL AND class_room_id = ?)` — sesuaikan dengan logika yang sama untuk menandai SK lama tidak aktif
- `SELECT MAX(bed_id) FROM sk_bed` — pastikan kolom `bed_id` ada
- Batch INSERT dengan `INSERT INTO sk_bed (...) VALUES (...)` — pastikan semua kolom yang di-insert ada di schema Anda. Kolom yang dipakai: `clinic_id`, `class_room_id`, `kelas`, `bed`, `id_tt_siranap`, `ruang_siranap`, `kelas_siranap`, `covid`, `siranap`, `jml_ruang_siranap`, `kodekelas`, `namakelas`, `namaruang`, `kris`, `kamar`, `org_unit_code`, `sk_no`, `tgl_berlaku`, `tgl_berakhir`, `created_at`, `updated_at`

### 9.3 Ringkasan Tabel dan Kolom yang Harus Ada di SIMRS Anda

| Tabel | Kolom yang Dipakai | Keterangan |
|---|---|---|
| `sk_bed` | `sk_no`, `class_room_id`, `kamar`, `namaruang`, `id_tt_siranap`, `ruang_siranap`, `jml_ruang_siranap`, `kelas_siranap`, `bed`, `covid`, `kodekelas`, `namakelas`, `kris`, `tgl_berlaku`, `tgl_berakhir`, `created_at`, `updated_at` | SK definitions dan mapping bed |
| `beds` | `bed_id`, `class_room_id`, `kamar`, `room_id`, `id_kelas`, `nm_kelas`, `id_perawatan`, `nm_perawatan`, `id_tt_siranap`, `id_siranap`, `deskripsi_siranap`, `covid` | Master data bed per ruangan |
| `status_covid` | `id_tt`, `status`, `konfirmasi`, `antrian` | Status COVID per TT |
| `pasien_visitation` (atau tabel pasien) | `no_registration`, `class_room_id`, `bed_id`, `keluar_id` | Data pasien yang sedang dirawat |

Jika SIMRS Anda tidak memiliki tabel `status_covid`, Anda bisa mengabaikan kolom tersebut di query dan meng-handle-nya di kode Go dengan memberikan nilai default.

### 9.4 Checklist Adaptasi

1. [ ] Ganti driver database di `go.mod` dan `main.go`
2. [ ] Adaptasi semua query T-SQL → syntax database target
3. [ ] Hapus semua `WITH (NOLOCK)` atau ganti dengan `FOR UPDATE` (PostgreSQL)
4. [ ] Ganti `IIF()` → `CASE WHEN`
5. [ ] Ganti `ISNULL()` → `COALESCE()`
6. [ ] Adaptasi `ISNULL(NULLIF(LTRIM(RTRIM(...)), ''), ...)` → `COALESCE(NULLIF(TRIM(...), ''), ...)`
7. [ ] Untuk `GetBedAvailability`: inline-kan subquery `TempRanap` langsung di main query jika database tidak mendukung CTE temp table (PostgreSQL/MySQL)
8. [ ] Ganti `BEGIN TRAN` / `COMMIT` / `ROLLBACK` transaksi ke sintaks database target
9. [ ] Validasi semua kolom `sk_bed`, `beds`, `status_covid`, dan tabel pasien ada di SIMRS Anda
10. [ ] Ubah nilai `keluar_id` di query `GetBedAvailability` sesuai kode "belum keluar" di SIMRS Anda
