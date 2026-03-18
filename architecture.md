# Arsitektur & Task List
## Sistem Bridging RS Online V3 — RSUD Sleman

> Sistem sinkronisasi data tempat tidur otomatis antara **SIMRS (SQL Server)** dan **RS Online Kemenkes**.

---

## 1. Stack Teknologi

| Komponen | Teknologi |
|---|---|
| Back-end | Golang 1.2x (Goroutines & Channels) |
| Configuration | Viper (manajemen `.env`) |
| Database | Microsoft SQL Server |
| Front-end | Tailwind CSS + Alpine.js |
| Automation | Ticker-based Worker (interval 2 jam) |

---

## 2. Arsitektur Worker Pool

Aplikasi menginisialisasi sebuah **Dispatcher** yang mengatur antrean tugas melalui _job queue_.

```
┌─────────────┐     ┌──────────────┐     ┌────────────────────────┐
│   Ticker    │────▶│  Dispatcher  │────▶│       Worker Pool      │
│ (setiap 2j) │     │  (Job Queue) │     │                        │
└─────────────┘     └──────────────┘     │  1. Ambil data SIMRS   │
                                         │  2. Kalkulasi bed       │
                                         │  3. Generate X-Timestamp│
                                         │  4. PUT → API Kemenkes  │
                                         └────────────────────────┘
```

### Komponen Utama

**Job**
- Mengambil data dari SQL Server menggunakan query `#temp_ranap` dan query utama.

**Worker**
- Menerima `Job` dari channel.
- Melakukan kalkulasi jumlah bed terisi vs. total.
- Menghasilkan header `X-Timestamp` (format UTC).
- Mengirim `PUT` request ke API Kemenkes.

**Ticker**
- Pemicu otomatis setiap **2 jam** (dapat dikonfigurasi via `.env`).

---

## 3. Tampilan Dashboard

Dashboard terdiri dari **3 Tab** utama:

### Tab 1 — Info Ruang
> View data real-time dari SIMRS (hasil query gabungan).

### Tab 2 — Master Referensi
> Sinkronisasi data referensi dari Kemenkes via method `GET`.

### Tab 3 — Operasional & Worker
> Panel pemantauan dan kontrol worker.

- 📋 Log status pengiriman otomatis
- ▶️ Tombol manual **"Sync Now"** (Manual Trigger)
- 🟢 Indikator status Worker (`Running` / `Idle`)

---

## 4. Rencana Tugas (Task List)

### Fase 1 — Setup & Viper Configuration

- [ ] Inisialisasi proyek dan install library (`viper`, `sqlserver`, `resty`)
- [ ] Buat file `config/config.go` untuk mapping file `.env`

---

### Fase 2 — Database & Query (Repository)

- [ ] Implementasi fungsi `GetBedAvailability`:
    - [ ] Jalankan **Query 1** — insert ke `#temp_ranap`
    - [ ] Jalankan **Query 2** — join `sk_bed` & `status_covid`
    - [ ] Pastikan data dibungkus dalam struct `BedSiranap`

---

### Fase 3 — Worker Pool & Automation

- [ ] Buat struktur `Worker` dan `Job` channel
- [ ] Implementasi `time.NewTicker` untuk interval 2 jam
- [ ] Logic pengiriman ke API Kemenkes dengan headers:
    - `X-rs-id`
    - `X-Timestamp`
    - `X-pass`

---

### Fase 4 — Front-end Monitoring

- [ ] Desain header **"RS Online Pelaporan Tempat Tidur RSUD Sleman"** (Tailwind CSS)
- [ ] Implementasi Tab System menggunakan Alpine.js
- [ ] Integrasi API lokal untuk menampilkan log aktivitas worker di Tab 3

---

### Fase 5 — Uji Coba & Testing

- [ ] Simulasi kegagalan jaringan → validasi **Retry Mechanism**
- [ ] Verifikasi format timestamp UTC sesuai petunjuk teknis Kemenkes

---

## Ringkasan Progres

| Fase | Deskripsi | Status |
|---|---|---|
| Fase 1 | Setup & Konfigurasi | ⬜ Belum mulai |
| Fase 2 | Database & Query | ⬜ Belum mulai |
| Fase 3 | Worker Pool | ⬜ Belum mulai |
| Fase 4 | Front-end Dashboard | ⬜ Belum mulai |
| Fase 5 | Testing | ⬜ Belum mulai |