package worker

import (
	"fmt"
	"net/http"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"

	"sirs-online/internal/logger"
	"sirs-online/internal/repository"
)

// runWorker adalah goroutine permanen yang mendengarkan job dari channel.
func runWorker(jobQueue <-chan Job) {
	for job := range jobQueue {
		processJob(job)
	}
}

// processJob menjalankan satu siklus sinkronisasi penuh:
//  1. Deteksi SK aktif (Query 0)
//  2. Ambil data ketersediaan bed (Query 1 + 2 dalam satu transaksi)
//  3. Kirim PUT per ruangan ke API Kemenkes (retry maks 2x)
//  4. Catat hasil ke file .log
func processJob(job Job) {
	// Set flag running agar Dispatcher tahu worker sedang aktif
	// Dispatcher yang inject job sudah memiliki referensi ke isRunning,
	// namun karena kita pisah file, kita akses melalui closure referensi di channel.
	// Trick: job membawa referensi Dispatcher lewat fungsi setter opsional.
	// Untuk simplisitas, kita gunakan flag global di package level.
	runningFlag.Store(true)
	defer runningFlag.Store(false)

	cfg := job.Config
	repo := job.Repo

	logger.Info("=== MULAI SIKLUS SYNC ===")

	// ─── Step 1: Deteksi SK aktif ────────────────────────────────────────────
	skNo, err := repo.GetActiveSKNo()
	if err != nil {
		logger.Error("Tidak ada SK aktif ditemukan di tabel sk_bed: %v", err)
		return
	}
	logger.Info("SK aktif terdeteksi: %s", skNo)

	// Update SK aktif yang dibagikan ke state global (dibaca oleh API handler)
	currentSKNo.Store(&skNo)

	// ─── Step 2: Ambil ketersediaan bed ──────────────────────────────────────
	beds, err := repo.GetBedAvailability(skNo)
	if err != nil {
		logger.Error("Gagal mengambil data bed: %v", err)
		return
	}
	logger.Info("Data bed berhasil diambil: %d ruangan", len(beds))

	// Update state in-memory yang dibagikan ke API handler
	SetBeds(beds)

	// ─── Step 3: Kirim PUT ke API Kemenkes per ruangan ───────────────────────
	// DisableKeepAlives: wajib agar setiap request mendapat fresh TCP connection.
	// Tanpa ini, setelah server menutup koneksi (mis. saat 404), retry berikutnya
	// akan mencoba reuse koneksi yang sudah mati dan menghasilkan error EOF.
	client := resty.New().
		SetTimeout(15 * time.Second).
		SetTransport(&http.Transport{
			DisableKeepAlives: true,
		})

	// URL PUT ke Kemenkes: fixed endpoint, id_tt dikirim melalui body JSON
	putURL := fmt.Sprintf("%s/Fasyankes", cfg.APIURL)

	for _, bed := range beds {
		// Timestamp di-generate per-bed agar selalu fresh
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		body := map[string]string{
			"id_tt":               bed.IDTTSiranap,
			"ruang":               bed.Siranap,
			"jumlah_ruang":        strconv.Itoa(bed.JmlRuang),
			"jumlah":              strconv.Itoa(bed.Jumlah),
			"terpakai":            strconv.Itoa(bed.Terisi),
			"terpakai_suspek":     strconv.Itoa(bed.Status),
			"terpakai_konfirmasi": strconv.Itoa(bed.Konfirmasi),
			"antrian":             strconv.Itoa(bed.Antrian),
			"prepare":             strconv.Itoa(bed.Prepare),
			"prepare_plan":        strconv.Itoa(bed.PreparePlan),
			"covid":               strconv.Itoa(bed.Covid),
		}

		success := false
		for attempt := 1; attempt <= cfg.RetryMax+1; attempt++ {
			resp, err := client.R().
				SetHeader("X-rs-id", cfg.APIRsID).
				SetHeader("X-pass", cfg.APIPass).
				SetHeader("X-Timestamp", timestamp).
				SetHeader("Content-Type", "application/json").
				SetBody(body).
				Put(putURL)

			if err == nil && resp.StatusCode() == 200 {
				logger.Info("[SUCCESS] Ruang %s (id_tt=%s) berhasil diupdate", bed.Siranap, bed.IDTTSiranap)
				success = true
				break
			}

			statusCode := 0
			if err == nil {
				statusCode = resp.StatusCode()
			}
			logger.Warn("[RETRY %d/%d] Ruang %s gagal — url: %s, status: %d, error: %v",
				attempt, cfg.RetryMax, bed.Siranap, putURL, statusCode, err)

			if attempt < cfg.RetryMax+1 {
				time.Sleep(5 * time.Second)
			}
		}

		if !success {
			logger.Error("[FAILED] Ruang %s (id_tt=%s) gagal diupdate setelah %d retry",
				bed.Siranap, bed.IDTTSiranap, cfg.RetryMax)
		}
	}

	logger.Info("=== SIKLUS SYNC SELESAI ===")
}

// runningFlag (atomic) — true jika worker sedang aktif memproses job
var runningFlag atomic.Bool

// IsWorkerRunning mengekspos status worker ke package lain (termasuk Dispatcher)
func IsWorkerRunning() bool {
	return runningFlag.Load()
}

// currentSKNo menyimpan sk_no aktif (atomic string via pointer)
var currentSKNo atomic.Pointer[string]

// GetActiveSKNoCurrent mengembalikan sk_no aktif yang sedang digunakan
func GetActiveSKNoCurrent() string {
	p := currentSKNo.Load()
	if p == nil {
		return ""
	}
	return *p
}

// bedsMu melindungi bedsState dari concurrent read/write
var bedsMu sync.RWMutex

// bedsState menyimpan data bed in-memory
var bedsState []repository.BedSiranap

// SetBeds memperbarui state in-memory (thread-safe)
func SetBeds(beds []repository.BedSiranap) {
	bedsMu.Lock()
	defer bedsMu.Unlock()
	bedsState = beds
}

// GetBeds mengembalikan snapshot data bed saat ini (thread-safe)
func GetBeds() []repository.BedSiranap {
	bedsMu.RLock()
	defer bedsMu.RUnlock()
	result := make([]repository.BedSiranap, len(bedsState))
	copy(result, bedsState)
	return result
}
