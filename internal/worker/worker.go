package worker

import (
	"encoding/json"
	"fmt"
	"strconv"
	"sync"
	"sync/atomic"
	"time"

	"github.com/go-resty/resty/v2"

	"sirs-online/config"
	"sirs-online/internal/logger"
	"sirs-online/internal/repository"
)

// ─── Struktur response GET /Fasyankes ────────────────────────────────────────

// fasyankesRecord merepresentasikan satu entri dari response GET /Fasyankes Kemenkes.
// Field id_t_tt adalah ID unik record di server Kemenkes yg dibutuhkan untuk PUT.
type fasyankesRecord struct {
	IDTtt string `json:"id_t_tt"`
	IDTt  string `json:"id_tt"`
	Ruang string `json:"ruang"`
}

// fasyankesResponse adalah pembungkus JSON response GET /Fasyankes
type fasyankesResponse struct {
	Fasyankes []fasyankesRecord `json:"fasyankes"`
}

// ─── processJob ──────────────────────────────────────────────────────────────

// processJob menjalankan satu siklus sinkronisasi penuh:
//  1. Deteksi SK aktif (Query 0)
//  2. Ambil data ketersediaan bed (Query 1 + 2 dalam satu transaksi)
//  3. Pre-fetch mapping id_t_tt dari Kemenkes GET /Fasyankes
//  4. Kirim PUT per ruangan ke API Kemenkes (retry maks RetryMax+1 kali)
//  5. Catat hasil ke file .log
func processJob(job Job) {
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
	currentSKNo.Store(&skNo)

	// ─── Step 2: Ambil ketersediaan bed ──────────────────────────────────────
	beds, err := repo.GetBedAvailability(skNo)
	if err != nil {
		logger.Error("Gagal mengambil data bed: %v", err)
		return
	}
	logger.Info("Data bed berhasil diambil: %d ruangan", len(beds))
	SetBeds(beds)

	// ─── Step 3: Init HTTP client (shared untuk pre-fetch & PUT) ─────────────
	client := NewKemenkesClient(cfg.TLSSkipVerify)

	// ─── Step 4: Pre-fetch mapping id_t_tt dari Kemenkes ─────────────────────
	// Key: "{id_tt}|{ruang}" → Value: id_t_tt (ID unik record di server Kemenkes)
	idTttMap, fetchErr := fetchFasyankesMapping(client, cfg)
	if fetchErr != nil {
		logger.Warn("Gagal mengambil mapping id_t_tt dari GET /Fasyankes: %v", fetchErr)
		logger.Warn("Sync tetap berjalan — ruangan yg id_t_tt-nya tidak ditemukan akan di-skip")
	} else {
		logger.Info("Mapping id_t_tt berhasil diambil: %d entri dari Kemenkes", len(idTttMap))
	}

	// ─── Step 5: Kirim PUT ke API Kemenkes per ruangan ───────────────────────
	putURL := fmt.Sprintf("%s/Fasyankes", cfg.APIURL)

	for _, bed := range beds {
		// Lookup id_t_tt dari mapping pre-fetch
		mapKey := bed.IDTTSiranap + "|" + bed.Siranap
		idTtt, found := idTttMap[mapKey]
		if !found {
			logger.Warn("[SKIP] Ruang %s (id_tt=%s) — id_t_tt tidak ditemukan di mapping Kemenkes",
				bed.Siranap, bed.IDTTSiranap)
			continue
		}

		// Timestamp di-generate per-bed agar selalu fresh
		timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

		body := map[string]string{
			"id_t_tt":             idTtt,
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
				respBody := truncateStr(string(resp.Body()), 300)
				logger.Info("[SUCCESS] Ruang %s (id_tt=%s, id_t_tt=%s) — response: %s",
					bed.Siranap, bed.IDTTSiranap, idTtt, respBody)
				success = true
				break
			}

			statusCode := 0
			respBody := ""
			if err == nil {
				statusCode = resp.StatusCode()
				respBody = truncateStr(string(resp.Body()), 300)
			}
			logger.Warn("[RETRY %d/%d] Ruang %s — url: %s, status: %d, body: %s, error: %v",
				attempt, cfg.RetryMax+1, bed.Siranap, putURL, statusCode, respBody, err)

			if attempt < cfg.RetryMax+1 {
				time.Sleep(5 * time.Second)
			}
		}

		if !success {
			logger.Error("[FAILED] Ruang %s (id_tt=%s, id_t_tt=%s) gagal diupdate setelah %d percobaan",
				bed.Siranap, bed.IDTTSiranap, idTtt, cfg.RetryMax+1)
		}
	}

	logger.Info("=== SIKLUS SYNC SELESAI ===")
}

// ─── fetchFasyankesMapping ───────────────────────────────────────────────────

// fetchFasyankesMapping melakukan GET ke {API_URL}/Fasyankes dan membangun mapping
// key "{id_tt}|{ruang}" → id_t_tt.
// id_t_tt adalah ID unik record di server Kemenkes yang wajib disertakan saat PUT.
func fetchFasyankesMapping(client *resty.Client, cfg *config.Config) (map[string]string, error) {
	url := fmt.Sprintf("%s/Fasyankes", cfg.APIURL)
	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	logger.Info("[PRE-FETCH] GET %s untuk mapping id_t_tt", url)

	resp, err := client.R().
		SetHeader("X-rs-id", cfg.APIRsID).
		SetHeader("X-pass", cfg.APIPass).
		SetHeader("X-Timestamp", timestamp).
		Get(url)

	if err != nil {
		return nil, fmt.Errorf("HTTP request gagal: %w", err)
	}

	if resp.StatusCode() != 200 {
		body := truncateStr(string(resp.Body()), 300)
		return nil, fmt.Errorf("status %d dari Kemenkes: %s", resp.StatusCode(), body)
	}

	logger.Info("[PRE-FETCH] Response %d (%d bytes)", resp.StatusCode(), len(resp.Body()))

	// Parse JSON response — coba dua kemungkinan struktur:
	// 1. {"fasyankes": [...]}   (key "fasyankes")
	// 2. Langsung array [...]
	var records []fasyankesRecord

	var wrapped fasyankesResponse
	if err := json.Unmarshal(resp.Body(), &wrapped); err == nil && len(wrapped.Fasyankes) > 0 {
		records = wrapped.Fasyankes
	} else {
		// Coba langsung sebagai array
		if err := json.Unmarshal(resp.Body(), &records); err != nil {
			body := truncateStr(string(resp.Body()), 500)
			return nil, fmt.Errorf("gagal parse JSON response: %w — body: %s", err, body)
		}
	}

	// Bangun mapping key "{id_tt}|{ruang}" → id_t_tt
	mapping := make(map[string]string, len(records))
	for _, rec := range records {
		if rec.IDTtt == "" {
			continue
		}
		key := rec.IDTt + "|" + rec.Ruang
		mapping[key] = rec.IDTtt
	}

	return mapping, nil
}

// ─── Helpers ─────────────────────────────────────────────────────────────────

// truncateStr memotong string agar log tidak terlalu panjang.
func truncateStr(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen] + "...[truncated]"
}

// ─── runWorker ───────────────────────────────────────────────────────────────

func runWorker(jobQueue <-chan Job) {
	for job := range jobQueue {
		processJob(job)
	}
}

// ─── State globals ───────────────────────────────────────────────────────────

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
