// Package main — test standalone untuk PUT update Tempat Tidur ke API RS Online Kemenkes.
//
// Program ini mereplikasi persis proses pengiriman yang ada di internal/worker/worker.go.
// Berguna untuk diagnosa langsung tanpa harus menjalankan full sync.
//
// Cara pakai:
//
//	go run test/put_tt/main.go
//	go run test/put_tt/main.go -id_tt=5 -ruang="Alamanda 1-Kamar 1" -jumlah=4 -terpakai=1
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"net/http"
	"os"
	"strconv"
	"strings"
	"time"

	"github.com/go-resty/resty/v2"
	"github.com/spf13/viper"
)

func main() {
	// ─── 1. Parse flag CLI ────────────────────────────────────────────────────
	idTT := flag.String("id_tt", "5", "ID tempat tidur (id_tt_siranap dari sk_bed)")
	ruang := flag.String("ruang", "Alamanda 1-Kamar 1", "Nama ruang/kamar sesuai Kemenkes")
	jumlahRuang := flag.String("jumlah_ruang", "1", "Jumlah ruang")
	jumlah := flag.String("jumlah", "4", "Jumlah bed tersedia")
	terpakai := flag.String("terpakai", "1", "Jumlah bed terpakai")
	terpakaISuspek := flag.String("terpakai_suspek", "0", "Terpakai suspek")
	terpakai_konfirmasi := flag.String("terpakai_konfirmasi", "0", "Terpakai konfirmasi")
	antrian := flag.String("antrian", "0", "Antrian")
	prepare := flag.String("prepare", "0", "Prepare")
	preparePlan := flag.String("prepare_plan", "0", "Prepare plan")
	covid := flag.String("covid", "0", "COVID (0 atau 1)")
	flag.Parse()

	// ─── 2. Load konfigurasi dari .env ───────────────────────────────────────
	viper.SetConfigFile(".env")
	viper.SetConfigType("env")
	viper.AutomaticEnv()
	if err := viper.ReadInConfig(); err != nil {
		fmt.Printf("[WARN] Tidak bisa membaca .env: %v\n", err)
		fmt.Println("       Pastikan dijalankan dari root direktori proyek: go run test/put_tt/main.go")
	}

	apiURL := viper.GetString("API_URL")
	rsID := viper.GetString("API_RS_ID")
	pass := viper.GetString("API_PASS")

	if apiURL == "" || rsID == "" || pass == "" {
		fmt.Println("[ERROR] Konfigurasi API tidak lengkap. Periksa .env (API_URL, API_RS_ID, API_PASS).")
		os.Exit(1)
	}

	// ─── 3. Bangun URL & body ─────────────────────────────────────────────────
	// URL sama persis dengan yang digunakan worker.go:
	// cfg.API.URL + "/Fasyankes\"  → id_tt dikirim di body, bukan di path URL
	putURL := apiURL + `/Fasyankes\`

	body := map[string]string{
		"id_tt":               *idTT,
		"ruang":               *ruang,
		"jumlah_ruang":        *jumlahRuang,
		"jumlah":              *jumlah,
		"terpakai":            *terpakai,
		"terpakai_suspek":     *terpakaISuspek,
		"terpakai_konfirmasi": *terpakai_konfirmasi,
		"antrian":             *antrian,
		"prepare":             *prepare,
		"prepare_plan":        *preparePlan,
		"covid":               *covid,
	}

	timestamp := strconv.FormatInt(time.Now().UTC().Unix(), 10)

	// ─── 4. Cetak ringkasan request ───────────────────────────────────────────
	bodyJSON, _ := json.MarshalIndent(body, "  ", "  ")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Println("  TEST PUT Tempat Tidur — API RS Online Kemenkes")
	fmt.Println(strings.Repeat("─", 60))
	fmt.Printf("  URL         : %s\n", putURL)
	fmt.Printf("  X-rs-id     : %s\n", rsID)
	fmt.Printf("  X-pass      : %s\n", pass)
	fmt.Printf("  X-Timestamp : %s\n", timestamp)
	fmt.Printf("  Body        :\n  %s\n", string(bodyJSON))
	fmt.Println(strings.Repeat("─", 60))

	// ─── 5. Kirim request (sama persis dengan worker.go) ─────────────────────
	// DisableKeepAlives: mencegah reuse koneksi mati → mencegah error EOF
	client := resty.New().
		SetTimeout(15 * time.Second).
		SetTransport(&http.Transport{
			DisableKeepAlives: true,
		})

	fmt.Println("  Mengirim request...")
	resp, err := client.R().
		SetHeader("X-rs-id", rsID).
		SetHeader("X-pass", pass).
		SetHeader("X-Timestamp", timestamp).
		SetHeader("Content-Type", "application/json").
		SetBody(body).
		Put(putURL)

	// ─── 6. Cetak hasil ───────────────────────────────────────────────────────
	fmt.Println(strings.Repeat("─", 60))
	if err != nil {
		fmt.Printf("  [ERROR] Request gagal: %v\n", err)
		fmt.Println(strings.Repeat("─", 60))
		os.Exit(1)
	}

	status := resp.StatusCode()
	respBody := string(resp.Body())

	fmt.Printf("  Status      : %d\n", status)
	fmt.Printf("  Response    : %s\n", respBody)
	fmt.Println(strings.Repeat("─", 60))

	if status == 200 {
		fmt.Printf("  ✓ [SUCCESS] id_tt=%s (%s) berhasil diupdate\n", *idTT, *ruang)
	} else {
		fmt.Printf("  ✗ [FAILED]  id_tt=%s (%s) — status: %d\n", *idTT, *ruang, status)
		os.Exit(1)
	}
	fmt.Println(strings.Repeat("─", 60))
}
